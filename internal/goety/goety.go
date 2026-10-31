package goety

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ddb "github.com/code-gorilla-au/goety/internal/dynamodb"
)

const (
	defaultBatchSize = 25
)

// New wires a Service. A nil progress callback is a no-op, so callers that
// only care about results can omit it.
func New(client DynamoClient, logger *slog.Logger, progress func(string), dryRun bool) Service {
	return Service{
		client:           client,
		dryRun:           dryRun,
		logger:           logger,
		fileReaderWriter: &OSFile{},
		progress:         progress,
	}
}

// reportProgress forwards a status update to the configured callback. It never
// blocks on I/O itself; a spinner callback only stores the latest message.
func (s Service) reportProgress(message string) {
	if s.progress == nil {
		return
	}

	s.progress(message)
}

// Purge all items from the given table
//
// Example:
//
//	Purge(ctx, "my-table", TableKeys{ PartitionKey: "pk", SortKey: "sk" })
func (s Service) Purge(ctx context.Context, tableName string, keys TableKeys) error {
	s.reportProgress(fmt.Sprintf("scanning table %s for items to purge", tableName))
	now := time.Now()

	deleted := 0

	input := &dynamodb.ScanInput{
		TableName:       &tableName,
		AttributesToGet: []string{keys.PartitionKey, keys.SortKey},
		Limit:           aws.Int32(defaultBatchSize),
	}

	if s.dryRun {
		s.logger.Debug("dry run enabled")
	}

	for out, err := range ddb.ScanAll(ctx, s.client, input) {
		if err != nil {
			return fmt.Errorf("scan items for purge: %w", err)
		}

		if len(out.Items) == 0 {
			break
		}

		if s.dryRun {
			prettyPrint(out.Items)
			continue
		}

		if _, err = s.client.BatchDeleteItems(ctx, tableName, out.Items); err != nil {
			return fmt.Errorf("delete items for purge: %w", err)
		}

		deleted += len(out.Items)

		s.reportProgress(fmt.Sprintf("deleted %d items", deleted))
	}

	since := time.Since(now)

	s.reportProgress(fmt.Sprintf("purge complete, deleted %d items, time taken [%v]", deleted, since))
	return nil
}

// Dump all items from the given table. Optionally specify a list of attributes to extract.
//
// Example:
//
//	Dump(ctx, "my-table", "path/to/file.json", []string{"attr1", "attr2"})
func (s Service) Dump(ctx context.Context, tableName string, path string, opts ...DumpOption) (err error) {
	s.reportProgress(fmt.Sprintf("dumping table %s to file %s", tableName, path))

	dumpOpts := WithDumpOptions(opts)

	itemsScanned := 0

	frw := dumpWriter{}

	if !s.dryRun {
		err = frw.Init(s.fileReaderWriter, path)
		if err != nil {
			return fmt.Errorf("open dump file: %w", err)
		}

		defer func() {
			dErr := frw.Close()
			if dErr != nil {
				err = errors.Join(err, dErr)
			}
		}()
	}

	scanInput := &dynamodb.ScanInput{
		TableName:                 &tableName,
		ProjectionExpression:      dumpOpts.ProjectedExpressions,
		FilterExpression:          dumpOpts.FilterExpression,
		ExpressionAttributeNames:  dumpOpts.FilterNameAttributes,
		ExpressionAttributeValues: dumpOpts.FilterNameValues,
		Limit:                     dumpOpts.Limit,
	}

	for output, scanErr := range ddb.ScanAll(ctx, s.client, scanInput) {
		if scanErr != nil {
			return fmt.Errorf("scan items for dump: %w", scanErr)
		}

		if len(output.Items) == 0 {
			break
		}

		if s.dryRun {
			items, transformErr := transformDumpOutput(output.Items, dumpOpts.RawOutput)
			if transformErr != nil {
				return fmt.Errorf("transform dump items: %w", transformErr)
			}

			prettyPrint(items)

		} else {
			if err = frw.WriteItems(output.Items, dumpOpts.RawOutput); err != nil {
				return fmt.Errorf("write dump items: %w", err)
			}
		}

		itemsScanned += len(output.Items)
		s.reportProgress(fmt.Sprintf("scanned %d items", itemsScanned))
	}

	s.reportProgress(fmt.Sprintf("dump complete, saved %d items to %s", itemsScanned, path))

	return
}

// Seed a table with items from a json file
//
// Example:
//
//	Seed(ctx, "my-table", "path/to/file.json")
func (s Service) Seed(ctx context.Context, tableName string, filePath string) (err error) {
	s.reportProgress(fmt.Sprintf("putting items to table %s", tableName))

	r, err := s.fileReaderWriter.Open(filePath)
	if err != nil {
		return fmt.Errorf("open seed file: %w", err)
	}

	defer func() {
		if cerr := r.Close(); cerr != nil {
			if err == nil {
				err = fmt.Errorf("close seed file: %w", cerr)
			}
		}
	}()

	dec := json.NewDecoder(r)

	token, err := dec.Token()
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}

	delim, ok := token.(json.Delim)
	if !ok || delim != '[' {
		return fmt.Errorf("expected JSON array, got %v", token)
	}

	itemsLoaded := 0
	batch := make([]map[string]types.AttributeValue, 0, defaultBatchSize)

	for dec.More() {
		var item map[string]any
		if err := dec.Decode(&item); err != nil {
			return fmt.Errorf("decode seed item: %w", err)
		}

		if s.dryRun {
			prettyPrint(item)
			itemsLoaded++
			continue
		}

		payload, err := attributevalue.MarshalMap(item)
		if err != nil {
			return fmt.Errorf("marshal seed item: %w", err)
		}

		s.logger.Debug("adding item to batch", "item", payload)
		batch = append(batch, payload)

		if len(batch) == defaultBatchSize {
			if _, err := s.client.BatchWriteItems(ctx, tableName, batch); err != nil {
				return fmt.Errorf("write seed items: %w", err)
			}
			itemsLoaded += len(batch)

			s.reportProgress(fmt.Sprintf("seeded %d items", itemsLoaded))

			batch = batch[:0]
		}
	}

	if !s.dryRun && len(batch) > 0 {
		if _, err := s.client.BatchWriteItems(ctx, tableName, batch); err != nil {
			return fmt.Errorf("write seed items: %w", err)
		}
		itemsLoaded += len(batch)
	}

	s.reportProgress(fmt.Sprintf("seed complete with %d items inserted", itemsLoaded))
	return nil
}

// prettyPrint - prints a pretty json representation of the given value
func prettyPrint(v any) {
	data, err := json.MarshalIndent(v, "\n", "  ")
	if err != nil {
		return
	}

	fmt.Println(string(data))
}

func transformDumpOutput(attrData []map[string]types.AttributeValue, rawOutput bool) ([]map[string]any, error) {
	if rawOutput {
		return ddb.ConvertAVValues(attrData)
	}
	return ddb.FlattenAttrList(attrData)
}

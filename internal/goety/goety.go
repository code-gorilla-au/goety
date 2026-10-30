package goety

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	for out, err := range ddb.ScanAll(ctx, s.client, input) {
		if err != nil {
			return fmt.Errorf("scan items for purge: %w", err)
		}

		if len(out.Items) == 0 {
			break
		}

		if s.dryRun {
			s.logger.Debug("dry run enabled")
			prettyPrint(out.Items)
			return nil
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
	var dryRunResult []map[string]any

	var w io.WriteCloser
	closed := false
	closeDumpFile := func() {
		if !closed {
			_, _ = w.Write([]byte("]"))
		}

		if cerr := w.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close dump file: %w", cerr)
		}
	}

	if !s.dryRun {
		w, err = s.openDumpFile(path)
		if err != nil {
			return err
		}
		defer closeDumpFile()
	}

	first := true
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
			dryRunResult = append(dryRunResult, items...)
		} else {
			if err = writeDumpItems(w, output.Items, dumpOpts.RawOutput, &first); err != nil {
				return fmt.Errorf("write dump items: %w", err)
			}
		}

		itemsScanned += len(output.Items)
		s.reportProgress(fmt.Sprintf("scanned %d items", itemsScanned))
	}

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(dryRunResult)
		return nil
	}

	closed = true
	if _, err = w.Write([]byte("]")); err != nil {
		return fmt.Errorf("write closing bracket to dump file: %w", err)
	}

	s.reportProgress(fmt.Sprintf("dump complete, saved %d items to %s", itemsScanned, path))
	s.logger.Info("dump complete", "items", itemsScanned)
	return nil
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
	var dryRunItems []map[string]any
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

	if s.dryRun {
		prettyPrint(dryRunItems)
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

func (s Service) openDumpFile(path string) (io.WriteCloser, error) {
	w, err := s.fileReaderWriter.Create(path, 0644)
	if err != nil {
		return nil, fmt.Errorf("create dump file: %w", err)
	}

	if _, err = w.Write([]byte("[")); err != nil {
		cErr := w.Close()
		if cErr != nil {
			return nil, fmt.Errorf("error closing: %w", cErr)
		}

		return nil, fmt.Errorf("write opening bracket to dump file: %w", err)
	}

	return w, nil
}

func writeDumpItems(w io.Writer, items []map[string]types.AttributeValue, rawOutput bool, first *bool) error {
	for _, item := range items {
		var v any
		var err error
		if rawOutput {
			v, err = ddb.ConvertAVValue(item)
		} else {
			v, err = ddb.FlattenAttrValue(item)
		}
		if err != nil {
			return err
		}

		data, err := json.Marshal(v)
		if err != nil {
			return err
		}

		if !*first {
			if _, err = w.Write([]byte(",")); err != nil {
				return err
			}
		}
		*first = false

		if _, err = w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func transformDumpOutput(attrData []map[string]types.AttributeValue, rawOutput bool) ([]map[string]any, error) {
	out := []map[string]any{}

	if !rawOutput {
		items, transformErr := ddb.FlattenAttrList(attrData)
		if transformErr != nil {
			return out, transformErr
		}
		out = append(out, items...)
		return out, nil
	}

	items, err := ddb.ConvertAVValues(attrData)
	if err != nil {
		return out, err
	}

	data, err := json.Marshal(items)
	if err != nil {
		return out, err
	}

	err = json.Unmarshal(data, &out)
	if err != nil {
		return out, err
	}

	return out, nil
}

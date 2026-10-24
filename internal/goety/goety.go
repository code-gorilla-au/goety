package goety

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ddb "github.com/code-gorilla-au/goety/internal/dynamodb"
	"github.com/code-gorilla-au/goety/internal/emitter"
)

const (
	defaultBatchSize = 25
)

func New(client DynamoClient, logger *slog.Logger, emitter emitter.MessagePublisher, dryRun bool) Service {
	return Service{
		client:           client,
		dryRun:           dryRun,
		logger:           logger,
		fileReaderWriter: &OSFile{},
		emitter:          emitter,
	}
}

// Purge all items from the given table
//
// Example:
//
//	Purge(ctx, "my-table", TableKeys{ PartitionKey: "pk", SortKey: "sk" })
func (s Service) Purge(ctx context.Context, tableName string, keys TableKeys) error {
	s.emitter.Publish(fmt.Sprintf("scanning table %s for items to purge", tableName))
	now := time.Now()

	done := false
	var err error
	var out *dynamodb.ScanOutput
	next := ddb.ScanIterator(ctx, s.client)

	deleted := 0

	for !done {
		out, err, done = next(&dynamodb.ScanInput{
			TableName:       &tableName,
			AttributesToGet: []string{keys.PartitionKey, keys.SortKey},
			Limit:           aws.Int32(defaultBatchSize),
		})
		if err != nil {
			s.logger.Error("could not scan table", "error", err)
			return err
		}

		if out == nil {
			break
		}

		if len(out.Items) == 0 {
			break
		}

		if s.dryRun {
			s.logger.Debug("dry run enabled")
			prettyPrint(out.Items)
			return nil
		}

		_, err = s.client.BatchDeleteItems(ctx, tableName, out.Items)
		if err != nil {
			s.logger.Error("could not batch delete items", "error", err)
			return err
		}
		deleted += len(out.Items)

		s.emitter.Publish(fmt.Sprintf("deleted %d items", deleted))

	}

	since := time.Since(now)

	s.emitter.Publish(fmt.Sprintf("purge complete, deleted %d items, time taken [%v]", deleted, since))
	return nil
}

// Dump all items from the given table. Optionally specify a list of attributes to extract.
//
// Example:
//
//	Dump(ctx, "my-table", "path/to/file.json", []string{"attr1", "attr2"})
func (s Service) Dump(ctx context.Context, tableName string, path string, opts ...QueryFuncOpts) (err error) {
	s.emitter.Publish(fmt.Sprintf("dumping table %s to file %s", tableName, path))

	queryOpts := WithQueryOptions(opts)

	done := false
	var output *dynamodb.ScanOutput
	next := ddb.ScanIterator(ctx, s.client)

	itemsScanned := 0
	var dryRunResult []map[string]any

	var w io.WriteCloser
	if !s.dryRun {
		w, err = s.openDumpFile(path)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := w.Close(); cerr != nil {
				s.logger.Error("could not close file", "error", cerr)
				if err == nil {
					err = cerr
				}
			}
		}()
	}

	first := true
	for !done {
		output, err, done = next(
			&dynamodb.ScanInput{
				TableName:                 &tableName,
				ProjectionExpression:      queryOpts.ProjectedExpressions,
				FilterExpression:          queryOpts.FilterExpression,
				ExpressionAttributeNames:  queryOpts.FilterNameAttributes,
				ExpressionAttributeValues: queryOpts.FilterNameValues,
			})
		if err != nil && !errors.Is(err, ddb.ErrNoItems) {
			s.logger.Error("could not scan table", "error", err)
			return err
		}

		if output == nil || len(output.Items) == 0 {
			break
		}

		if s.dryRun {
			items, transformErr := transformDumpOutput(output.Items, queryOpts.RawOutput)
			if transformErr != nil {
				s.logger.Error("could not transform items", "error", transformErr)
				return transformErr
			}
			dryRunResult = append(dryRunResult, items...)
		} else {
			if err = writeDumpItems(w, output.Items, queryOpts.RawOutput, &first); err != nil {
				s.logger.Error("could not write items", "error", err)
				return err
			}
		}

		itemsScanned += len(output.Items)
		s.emitter.Publish(fmt.Sprintf("scanned %d items", itemsScanned))
	}

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(dryRunResult)
		return nil
	}

	if _, err = w.Write([]byte("]")); err != nil {
		s.logger.Error("could not write file", "error", err)
		return err
	}

	s.emitter.Publish(fmt.Sprintf("dump complete, saved %d items to %s", itemsScanned, path))
	s.logger.Info("dump complete", "items", itemsScanned)
	return nil
}

// Seed a table with items from a json file
//
// Example:
//
//	Seed(ctx, "my-table", "path/to/file.json")
func (s Service) Seed(ctx context.Context, tableName string, filePath string) (err error) {
	s.emitter.Publish(fmt.Sprintf("putting items to table %s", tableName))

	r, err := s.fileReaderWriter.Open(filePath)
	if err != nil {
		s.logger.Error("could not open file", "error", err)
		return err
	}

	defer func() {
		if cerr := r.Close(); cerr != nil {
			s.logger.Error("could not close file", "error", cerr)
			if err == nil {
				err = cerr
			}
		}
	}()

	dec := json.NewDecoder(r)

	token, err := dec.Token()
	if err != nil {
		s.logger.Error("could not read file", "error", err)
		return err
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
			s.logger.Error("could not decode item", "error", err)
			return err
		}

		if s.dryRun {
			prettyPrint(item)
			itemsLoaded++
			continue
		}

		payload, err := attributevalue.MarshalMap(item)
		if err != nil {
			s.logger.Error("could not marshal item", "error", err)
			return err
		}

		s.logger.Debug("adding item to batch", "item", payload)
		batch = append(batch, payload)

		if len(batch) == defaultBatchSize {
			if _, err := s.client.BatchWriteItems(ctx, tableName, batch); err != nil {
				s.logger.Error("could not batch write items", "error", err)
				return err
			}
			itemsLoaded += len(batch)
			s.emitter.Publish(fmt.Sprintf("seeded %d items", itemsLoaded))
			batch = batch[:0]
		}
	}

	if !s.dryRun && len(batch) > 0 {
		if _, err := s.client.BatchWriteItems(ctx, tableName, batch); err != nil {
			s.logger.Error("could not batch write items", "error", err)
			return err
		}
		itemsLoaded += len(batch)
	}

	s.emitter.Publish(fmt.Sprintf("seed complete with %d items inserted", itemsLoaded))
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
		s.logger.Error("could not create file", "error", err)
		return nil, err
	}

	if _, err = w.Write([]byte("[")); err != nil {
		s.logger.Error("could not write file", "error", err)
		if cErr := w.Close(); cErr != nil {
			s.logger.Error("could not close file", "error", cErr)
		}
		return nil, err
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

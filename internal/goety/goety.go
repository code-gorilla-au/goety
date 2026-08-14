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
		client:  client,
		dryRun:  dryRun,
		logger:  logger,
		emitter: emitter,
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
func (s Service) Dump(ctx context.Context, tableName string, writer Writer, opts ...QueryFuncOpts) error {
	s.emitter.Publish(fmt.Sprintf("dumping table %s", tableName))

	encoder := json.NewEncoder(writer)
	_, err := fmt.Fprint(writer, "[\n")
	if err != nil {
		s.logger.Error("Error writing to buffer:", "error", err)
		return err
	}

	defer func() {
		_, err := fmt.Fprint(writer, "\n]")
		if err != nil {
			s.logger.Error("Error writing to buffer:", "error", err)
			return
		}
	}()

	queryOpts := WithQueryOptions(opts)

	done := false
	var output *dynamodb.ScanOutput
	next := ddb.ScanIterator(ctx, s.client)

	itemsScanned := 0

	firstItem := true

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

		if output == nil {
			break
		}

		items, err := transformDumpOutput(output.Items, queryOpts.RawOutput)
		if err != nil {
			s.logger.Error("could not transform items", "error", err)
			return err
		}

		for _, item := range items {
			if s.dryRun {
				s.logger.Debug("dry run enabled")
				prettyPrint(item)
				continue
			}

			if !firstItem {
				_, err = fmt.Fprint(writer, ",\n")
				if err != nil {
					s.logger.Error("could not write to buffer", "error", err)
					return err
				}
			}
			firstItem = false

			err = encoder.Encode(item)
			if err != nil {
				s.logger.Error("could not encode items", "error", err)
				return err
			}
		}

		itemsScanned += len(items)
		s.emitter.Publish(fmt.Sprintf("scanned %d items", itemsScanned))

	}

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		return nil
	}

	message := fmt.Sprintf("saving %d items ", itemsScanned)
	s.emitter.Publish(message)

	s.emitter.Publish("dump complete")
	s.logger.Info("dump complete", "items", itemsScanned)
	return nil
}

// Seed a table with items from a json file
//
// Example:
//
//	Seed(ctx, "my-table", "path/to/file.json")
func (s Service) Seed(ctx context.Context, tableName string, reader io.Reader) error {
	s.emitter.Publish(fmt.Sprintf("putting items to table %s", tableName))

	decoder := json.NewDecoder(reader)
	token, err := decoder.Token()
	if err != nil {
		s.logger.Error("could not read starting token", "error", err)
		return err
	}

	delim, ok := token.(json.Delim)
	if !ok || delim != '[' {
		return errors.New("expected top-level JSON array")
	}

	if s.dryRun {
		s.logger.Debug("dry run enabled")
	}

	itemCount := 0
	for decoder.More() {
		var item map[string]any
		err = decoder.Decode(&item)
		if err != nil {
			s.logger.Error("could not decode item", "error", err)
			return err
		}

		itemCount++

		if s.dryRun {
			s.logger.Debug("dry run enabled")
			prettyPrint(item)
			continue
		}

		payload, err := attributevalue.MarshalMap(item)
		if err != nil {
			s.logger.Error("could not marshal item", "error", err)
			return err
		}

		s.logger.Debug("putting item", "item", payload)

		if _, err := s.client.Put(ctx, &dynamodb.PutItemInput{
			TableName: &tableName,
			Item:      payload,
		}); err != nil {
			return err
		}
	}

	s.emitter.Publish(fmt.Sprintf("seed complete with %d items inserted", itemCount))
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

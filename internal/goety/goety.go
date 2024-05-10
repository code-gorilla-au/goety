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
	ddb "github.com/code-gorilla-au/goety/internal/dynamodb"
	"github.com/code-gorilla-au/goety/internal/emitter"
)

const (
	defaultBatchSize = 25
)

func New(client DynamoClient, logger *slog.Logger, emitter emitter.MessagePublisher, dryRun bool) Service {
	return Service{
		client:     client,
		dryRun:     dryRun,
		logger:     logger,
		fileWriter: &WriteFile{},
		emitter:    emitter,
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
func (s Service) Dump(ctx context.Context, tableName string, path string, opts ...QueryFuncOpts) error {
	s.emitter.Publish(fmt.Sprintf("dumping table %s to file %s", tableName, path))

	queryOpts := WithQueryOptions(opts)

	done := false
	var err error
	var output *dynamodb.ScanOutput
	next := ddb.ScanIterator(ctx, s.client)

	result := []map[string]any{}

	itemsScanned := 0

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

		items, transformErr := ddb.FlattenAttrList(output.Items)
		if transformErr != nil {
			s.logger.Error("could not transform items", "error", transformErr)
			return transformErr
		}
		result = append(result, items...)

		itemsScanned += len(output.Items)
		s.emitter.Publish(fmt.Sprintf("scanned %d items", itemsScanned))

	}

	s.emitter.Publish(fmt.Sprintf("scanned %d items", len(result)))

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(result)
		return nil
	}

	message := fmt.Sprintf("saving %d items to file ", len(result)) + path
	s.emitter.Publish(message)
	data, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		s.logger.Error("could not marshal items", "error", marshalErr)
		return marshalErr
	}

	if fileErr := s.fileWriter.WriteFile(path, data, 0644); fileErr != nil {
		s.logger.Error("could not write file", "error", fileErr)
		return fileErr
	}

	s.emitter.Publish("dump complete")
	s.logger.Info("dump complete", "items", itemsScanned)
	return nil
}

// Seed a table with items from a json file
//
// Example:
//
//	Seed(ctx, "my-table", "path/to/file.json")
func (s Service) Seed(ctx context.Context, tableName string, filePath string) error {
	s.emitter.Publish(fmt.Sprintf("putting items to table %s", tableName))

	data, err := s.fileWriter.ReadFile(filePath)
	if err != nil {
		s.logger.Error("could not read file", "error", err)
		return err
	}

	itemList := []map[string]any{}
	if err := json.Unmarshal(data, &itemList); err != nil {
		s.logger.Error("could not unmarshal file", "error", err)
		return err
	}

	s.emitter.Publish(fmt.Sprintf("%d items to be loaded into table %s", len(itemList), tableName))

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(itemList)
		return nil
	}

	for _, item := range itemList {
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

	s.emitter.Publish(fmt.Sprintf("seed complete with %d items inserted", len(itemList)))
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

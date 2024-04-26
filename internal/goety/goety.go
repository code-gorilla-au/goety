package goety

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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

	items, err := s.client.ScanAll(ctx, &dynamodb.ScanInput{
		TableName:       &tableName,
		AttributesToGet: []string{keys.PartitionKey, keys.SortKey},
	})
	if err != nil {
		s.logger.Error("could not scan table", "error", err)
		return err
	}

	s.emitter.Publish(fmt.Sprintf("items %d scanned, beginning purge", len(items)))

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(items)
		return nil
	}

	start := 0
	end := defaultBatchSize
	deleted := 0

	for start < len(items) {

		if end > len(items) {
			end = len(items)
		}

		batchItems := items[start:end]

		s.logger.Debug(fmt.Sprintf("batch delete %d items", len(batchItems)))
		_, err = s.client.BatchDeleteItems(ctx, tableName, batchItems)
		if err != nil {
			s.logger.Error("could not batch delete items", "error", err)
			return err
		}

		deleted += len(batchItems)
		start = end
		end += defaultBatchSize
	}

	s.emitter.Publish(fmt.Sprintf("purge complete, deleted %d items", deleted))
	return nil
}

// Dump all items from the given table. Optionally specify a list of attributes to extract.
//
// Example:
//
//	Dump(ctx, "my-table", "path/to/file.json", []string{"attr1", "attr2"})
func (s Service) Dump(ctx context.Context, tableName string, path string, attrs ...string) error {
	s.emitter.Publish(fmt.Sprintf("dumping table %s to file %s", tableName, path))

	var projExp *string

	if len(attrs) > 0 {
		projExp = aws.String(strings.Join(attrs, ", "))
	}

	items, err := s.client.ScanAll(ctx, &dynamodb.ScanInput{
		TableName:            &tableName,
		ProjectionExpression: projExp,
	})
	if err != nil {
		s.logger.Error("could not scan table", "error", err)
		return err
	}

	s.emitter.Publish(fmt.Sprintf("scanned %d items", len(items)))

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(items)
		return nil
	}

	message := fmt.Sprintf("saving %d items to file ", len(items)) + path
	s.emitter.Publish(message)
	data, err := json.Marshal(items)
	if err != nil {
		s.logger.Error("could not marshal items", "error", err)
		return err
	}

	if err := s.fileWriter.WriteFile(path, data, 0644); err != nil {
		s.logger.Error("could not write file", "error", err)
		return err
	}

	s.emitter.Publish("dump complete")
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

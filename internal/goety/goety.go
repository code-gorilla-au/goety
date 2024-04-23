package goety

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

const (
	defaultBatchSize = 25
)

func New(client DynamoClient, logger *slog.Logger, dryRun bool) Service {
	return Service{
		client:     client,
		dryRun:     dryRun,
		logger:     logger,
		fileWriter: &WriteFile{},
	}
}

// Purge all items from the given table
//
// Example:
//
//	Purge(ctx, "my-table", TableKeys{ PartitionKey: "pk", SortKey: "sk" })
func (s Service) Purge(ctx context.Context, tableName string, keys TableKeys) error {
	s.logger.Debug("running purge")

	items, err := s.client.ScanAll(ctx, &dynamodb.ScanInput{
		TableName:       &tableName,
		AttributesToGet: []string{keys.PartitionKey, keys.SortKey},
	})
	if err != nil {
		s.logger.Error("could not scan table", "error", err)
		return err
	}

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(items)
		return nil
	}

	s.logger.Debug(fmt.Sprintf("purging %d items", len(items)))

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

	s.logger.Debug(fmt.Sprintf("purge complete, deleted: %d", deleted))
	return nil
}

// Dump all items from the given table
func (s Service) Dump(ctx context.Context, tableName string, path string) error {
	s.logger.Debug("running dump")

	items, err := s.client.ScanAll(ctx, &dynamodb.ScanInput{
		TableName: &tableName,
	})
	if err != nil {
		s.logger.Error("could not scan table", "error", err)
		return err
	}

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(items)
		return nil
	}

	s.logger.Debug("saving to file", "filePath", path)
	data, err := json.Marshal(items)
	if err != nil {
		s.logger.Error("could not marshal items", "error", err)
		return err
	}

	if err := s.fileWriter.WriteFile(path, data, 0644); err != nil {
		s.logger.Error("could not write file", "error", err)
		return err
	}

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

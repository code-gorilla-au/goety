package dynamodb

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/config"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// NewClient - creates a new opinionated dynamodb client
func NewClient(ctx context.Context, region string, endpoint string, logger *slog.Logger) (*Client, error) {
	client := Client{logger: logger}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return &client, fmt.Errorf("load AWS config: %w", err)
	}

	db := ddb.NewFromConfig(cfg, func(o *ddb.Options) {
		o.Region = region
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
		}
	})
	client.db = db

	return &client, nil
}

// Scan - scans a dynamodb table
func (c *Client) Scan(ctx context.Context, input *ddb.ScanInput) (*ddb.ScanOutput, error) {
	output, err := c.db.Scan(ctx, input)
	if err != nil {
		return output, fmt.Errorf("scan table: %w", err)
	}

	return output, nil
}

// Put - puts an item into a dynamodb table
func (c *Client) Put(ctx context.Context, input *ddb.PutItemInput) (*ddb.PutItemOutput, error) {
	return c.db.PutItem(ctx, input)
}

// BatchWriteItems - writes items in a batch. Note, max size is 25 items within a batch.
func (c *Client) BatchWriteItems(ctx context.Context, tableName string, items []map[string]types.AttributeValue) (*ddb.BatchWriteItemOutput, error) {
	requests := make([]types.WriteRequest, 0, len(items))

	for _, item := range items {
		c.logger.Debug("adding item to batch write", "item", JSONStringify(item))

		requests = append(requests, types.WriteRequest{
			PutRequest: &types.PutRequest{Item: item},
		})
	}

	return c.batchWriteItem(ctx, tableName, requests, "write")
}

// BatchDeleteItems - deletes items in a batch. Note, max size is 25 items within a batch.
func (c *Client) BatchDeleteItems(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*ddb.BatchWriteItemOutput, error) {
	requests := make([]types.WriteRequest, 0, len(keys))

	for _, key := range keys {
		c.logger.Debug("adding key to batch delete", "key", JSONStringify(key))

		requests = append(requests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{Key: key},
		})
	}

	return c.batchWriteItem(ctx, tableName, requests, "delete")
}

func (c *Client) batchWriteItem(ctx context.Context, tableName string, requests []types.WriteRequest, op string) (*ddb.BatchWriteItemOutput, error) {
	if len(requests) == 0 {
		return &ddb.BatchWriteItemOutput{}, nil
	}

	input := ddb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{tableName: requests},
	}

	if c.dryRun {
		c.logger.Debug(fmt.Sprintf("dry run enabled, skipping batch %s", op), "items", JSONStringify(input))
		return &ddb.BatchWriteItemOutput{}, nil
	}

	output, err := c.db.BatchWriteItem(ctx, &input)
	if err != nil {
		return output, fmt.Errorf("batch %s items: %w", op, err)
	}

	unprocessedItems := output.UnprocessedItems

	for len(unprocessedItems) > 0 {
		unprocessedInput := ddb.BatchWriteItemInput{RequestItems: unprocessedItems}

		unprocessedOutput, uErr := c.db.BatchWriteItem(ctx, &unprocessedInput)
		if uErr != nil {
			return unprocessedOutput, fmt.Errorf("batch %s items: %w", op, uErr)
		}

		unprocessedItems = unprocessedOutput.UnprocessedItems
	}

	return output, nil
}

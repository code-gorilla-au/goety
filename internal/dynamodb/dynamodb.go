package dynamodb

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/goety/internal/logging"
)

// NewClient - creates a new opinionated dynamodb client
func NewClient(ctx context.Context, region string, endpoint string) (*Client, error) {
	ops := func(o *ddb.Options) {
		o.Region = region
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
		}
	}
	return NewWith(ctx, func(lo *config.LoadOptions) error { return nil }, ops)
}

// NewWith - creates a new dynamodb client with exposed functional options.
// Use this client if you wish to have flexibility with some of the more advanced options.
func NewWith(ctx context.Context, configOpts func(*config.LoadOptions) error, dbOpts ...func(*ddb.Options)) (*Client, error) {

	client := Client{
		logger: logging.FromContext(ctx),
	}

	cfg, err := config.LoadDefaultConfig(ctx, configOpts)
	if err != nil {
		return &client, err
	}

	db := ddb.NewFromConfig(cfg, dbOpts...)
	client.db = db

	return &client, nil
}

// Scan - scans a dynamodb table
func (c *Client) Scan(ctx context.Context, input *ddb.ScanInput) (*ddb.ScanOutput, error) {
	output, err := c.db.Scan(ctx, input)
	if err != nil {
		c.logger.Error("could not scan table", "error", err)
		return output, err
	}

	return output, nil
}

// Put - puts an item into a dynamodb table
func (c *Client) Put(ctx context.Context, input *ddb.PutItemInput) (*ddb.PutItemOutput, error) {
	return c.db.PutItem(ctx, input)
}

// BatchDeleteItems - deletes items in a batch Note, max size is 25 items within a batch
func (c *Client) BatchDeleteItems(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*ddb.BatchWriteItemOutput, error) {
	txnWrite := []types.WriteRequest{}

	for _, key := range keys {
		c.logger.Debug("adding key to batch delete", "key", JSONStringify(key))
		txnWrite = append(txnWrite, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: key,
			},
		})
	}

	input := ddb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: txnWrite,
		},
	}

	if c.dryRun {
		c.logger.Debug("dry run enabled, skipping batch delete", "items", JSONStringify(input))
		return &ddb.BatchWriteItemOutput{}, nil
	}

	output, err := c.db.BatchWriteItem(ctx, &input)
	if err != nil {
		c.logger.Error("could not batch delete items", "error", err)
		return output, err
	}

	if output.UnprocessedItems == nil {
		c.logger.Debug("batch delete complete")
		return output, nil
	}

	c.logger.Debug("unprocessed items detected, processing")

	unprocessedItems := output.UnprocessedItems

	for len(unprocessedItems) > 0 {
		unprocessedInput := ddb.BatchWriteItemInput{
			RequestItems: unprocessedItems,
		}

		unprocessedOutput, err := c.db.BatchWriteItem(ctx, &unprocessedInput)
		if err != nil {
			c.logger.Error("could not batch delete items", "error", err)
			return unprocessedOutput, err
		}

		unprocessedItems = unprocessedOutput.UnprocessedItems
	}
	return output, err
}

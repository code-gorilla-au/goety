package dynamodb

import (
	"context"
	"fmt"

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

func (c *Client) Scan(ctx context.Context, input *ddb.ScanInput) (*ddb.ScanOutput, error) {
	output, err := c.db.Scan(ctx, input)
	if err != nil {
		c.logger.Error("could not scan table", "error", err)
		return output, err
	}

	if len(output.Items) == 0 {
		c.logger.Error("no items returned")
		return output, ErrNoItems
	}
	return output, nil
}

func (c *Client) ScanAll(ctx context.Context, input *ddb.ScanInput) ([]map[string]types.AttributeValue, error) {
	results := []map[string]types.AttributeValue{}

	var lastEvaluatedKey map[string]types.AttributeValue

	for {

		c.logger.Debug(fmt.Sprintf("scanning with lastEvaluatedKey: %s", JSONStringify(lastEvaluatedKey)))

		input.ExclusiveStartKey = lastEvaluatedKey
		output, err := c.db.Scan(ctx, input)
		if err != nil {
			c.logger.Error("could not scan table", "error", err)
			return results, err
		}

		lastEvaluatedKey = output.LastEvaluatedKey

		c.logger.Debug(fmt.Sprintf("batch scan count: %d", len(output.Items)))
		results = append(results, output.Items...)

		if lastEvaluatedKey == nil {
			c.logger.Debug("scan complete")
			break
		}
	}

	c.logger.Debug(fmt.Sprintf("total scan count: %d", len(results)))
	return results, nil

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

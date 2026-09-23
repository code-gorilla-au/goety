package goety

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ddb "github.com/code-gorilla-au/goety/internal/dynamodb"
)

//go:generate moq -rm -stub -out mocks_test.go . DynamoClient
type DynamoClient interface {
	Put(ctx context.Context, input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error)
	Scan(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error)
	BatchDeleteItems(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error)
}

var _ DynamoClient = (*ddb.Client)(nil)

type Writer interface {
	io.Writer
}

type Emitter interface {
	Publish(msg string)
}

package goety

import (
	"context"
	"io/fs"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ddb "github.com/code-gorilla-au/goety/internal/dynamodb"
)

//go:generate moq -rm -stub -out mocks_test.go . DynamoClient
type DynamoClient interface {
	ScanAll(ctx context.Context, input *dynamodb.ScanInput) ([]map[string]types.AttributeValue, error)
	BatchDeleteItems(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error)
}

var _ DynamoClient = (*ddb.Client)(nil)

type fileWriter interface {
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

type Emitter interface {
	Publish(msg string)
}

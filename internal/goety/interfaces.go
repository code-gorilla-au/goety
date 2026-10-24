package goety

import (
	"context"
	"io"
	"io/fs"

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

type FileReader interface {
	Open(name string) (io.ReadCloser, error)
}

type FileWriter interface {
	Create(name string, perm fs.FileMode) (io.WriteCloser, error)
}

type FileReadWriter interface {
	FileReader
	FileWriter
}

type Emitter interface {
	Publish(msg string)
}

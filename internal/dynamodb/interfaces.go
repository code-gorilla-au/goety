package dynamodb

import (
	"context"

	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type ddbScanner interface {
	Scan(ctx context.Context, params *ddb.ScanInput, optFns ...func(*ddb.Options)) (*ddb.ScanOutput, error)
}

//go:generate moq -rm -stub -out mocks_test.go . ddbClient
type ddbClient interface {
	ddbScanner
	BatchWriteItem(ctx context.Context, params *ddb.BatchWriteItemInput, optFns ...func(*ddb.Options)) (*ddb.BatchWriteItemOutput, error)
}

package goety

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/goety/internal/logging"
	"github.com/code-gorilla-au/odize"
)

type mockEmitter struct {
	publishFunc func(message string)
}

func (m *mockEmitter) Publish(message string) {}

func TestService_Purge(t *testing.T) {
	var client DynamoClientMock
	var service Service
	logger := logging.New(true)
	ctx := logging.WithContext(context.Background(), logger)

	callScanAll := 0
	callBatchDelete := 0

	group := odize.NewGroup(t, nil)

	group.BeforeEach(func() {

		client = DynamoClientMock{
			ScanFunc: func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				callScanAll++
				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{
						{
							"pk": &types.AttributeValueMemberS{Value: "pk"},
							"sk": &types.AttributeValueMemberS{Value: "sk"},
						},
					},
				}, nil
			},
			BatchDeleteItemsFunc: func(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
				callBatchDelete++
				return &dynamodb.BatchWriteItemOutput{}, nil
			},
		}

		service = Service{
			client: &client,
			dryRun: false,
			logger: logger,
			emitter: &mockEmitter{
				publishFunc: func(message string) {},
			},
		}
	})

	group.AfterEach(func() {
		callScanAll = 0
		callBatchDelete = 0
	})

	err := group.
		Test("should purge items", func(t *testing.T) {
			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			odize.AssertEqual(t, 1, callBatchDelete)
		}).
		Test("should not delete items on dry run", func(t *testing.T) {
			service.dryRun = true

			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			odize.AssertEqual(t, 0, callBatchDelete)
		}).
		Test("should return error if scan fails", func(t *testing.T) {
			expectedErr := errors.New("scan all error")
			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				return nil, expectedErr
			}

			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertTrue(t, errors.Is(err, expectedErr))
		}).
		Test("should return error if batch write fails", func(t *testing.T) {
			expectedErr := errors.New("batch write error")
			client.BatchDeleteItemsFunc = func(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
				return nil, expectedErr
			}

			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertTrue(t, errors.Is(err, expectedErr))
		}).
		Test("should not fail if scan has no items", func(t *testing.T) {

			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{}, nil
			}

			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, 0, callBatchDelete)
		}).
		Run()

	odize.AssertNoError(t, err)

}

type mockWriteFile struct {
	writeFileFunc func(filename string, data []byte) error
	readFileFunc  func(filename string) ([]byte, error)
}

func (m *mockWriteFile) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return m.writeFileFunc(name, data)
}

func (m *mockWriteFile) ReadFile(name string) ([]byte, error) {
	return m.readFileFunc(name)
}

func TestService_Dump(t *testing.T) {
	var client DynamoClientMock
	var service Service
	var fileWriter mockWriteFile
	logger := logging.New(true)
	ctx := logging.WithContext(context.Background(), logger)

	callScanAll := 0
	callWriteFile := 0

	group := odize.NewGroup(t, nil)

	group.BeforeEach(func() {

		client = DynamoClientMock{
			ScanFunc: func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				callScanAll++
				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{
						{
							"pk": &types.AttributeValueMemberS{Value: "pk"},
							"sk": &types.AttributeValueMemberS{Value: "sk"},
						},
					},
				}, nil
			},
		}

		fileWriter = mockWriteFile{
			writeFileFunc: func(filename string, data []byte) error {
				fmt.Println("writing file", string(data))
				callWriteFile++
				return nil
			},
		}

		service = Service{
			client:     &client,
			dryRun:     false,
			logger:     logger,
			fileWriter: &fileWriter,
			emitter: &mockEmitter{
				publishFunc: func(message string) {},
			},
		}
	})

	group.AfterEach(func() {
		callScanAll = 0
		callWriteFile = 0
	})

	err := group.
		Test("should dump items", func(t *testing.T) {
			err := service.Dump(ctx, "my-table", "path")
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
		}).
		Test("should dump items on dry run", func(t *testing.T) {
			service.dryRun = true

			err := service.Dump(ctx, "my-table", "path")
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			odize.AssertEqual(t, 0, callWriteFile)
		}).
		Test("should output", func(t *testing.T) {
			fileWriter = mockWriteFile{
				writeFileFunc: func(filename string, data []byte) error {
					callWriteFile++
					odize.AssertEqual(t, "[{\"pk\":\"pk\",\"sk\":\"sk\"}]", string(data))
					return nil
				},
			}

			err := service.Dump(ctx, "my-table", "path")
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			odize.AssertEqual(t, 1, callWriteFile)
		}).
		Test("should output raw", func(t *testing.T) {
			fileWriter = mockWriteFile{
				writeFileFunc: func(filename string, data []byte) error {
					callWriteFile++
					odize.AssertEqual(t, `[{"pk":{"S":"pk"},"sk":{"S":"sk"}}]`, string(data))
					return nil
				},
			}

			err := service.Dump(ctx, "my-table", "path", WithRawOutput(true))
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			odize.AssertEqual(t, 1, callWriteFile)
		}).
		Test("should return error if scan fails", func(t *testing.T) {
			expectedErr := errors.New("scan all error")
			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				return nil, expectedErr
			}

			err := service.Dump(ctx, "my-table", "path")
			odize.AssertTrue(t, errors.Is(err, expectedErr))
		}).
		Test("should return error if file write fails", func(t *testing.T) {
			expectedErr := errors.New("write file error")
			fileWriter.writeFileFunc = func(filename string, data []byte) error {
				return expectedErr
			}

			err := service.Dump(ctx, "my-table", "path")
			fmt.Println("ooooo", err)
			odize.AssertTrue(t, errors.Is(err, expectedErr))
		}).
		Test("should dump items with attributes", func(t *testing.T) {
			attrExp := []string{"attr1", "attr2"}

			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				odize.AssertEqual(t, "attr1, attr2", *input.ProjectionExpression)

				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{
						{
							"pk": &types.AttributeValueMemberS{Value: "pk"},
							"sk": &types.AttributeValueMemberS{Value: "sk"},
						},
					},
				}, nil

			}

			err := service.Dump(ctx, "my-table", "path", WithAttrs(attrExp))
			odize.AssertNoError(t, err)
		}).
		Run()

	odize.AssertNoError(t, err)

}

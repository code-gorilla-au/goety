package goety

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/goety/internal/logging"
	"github.com/code-gorilla-au/odize"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

type mockEmitter struct {
	publishFunc func(message string)
}

func (m *mockEmitter) Publish(message string) {}

func (m *mockEmitter) PublishBlocking(message string) {}

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
	writeFileFunc func(filename string, data []byte, perm fs.FileMode) error
	readFileFunc  func(filename string) ([]byte, error)
}

func (m *mockWriteFile) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return m.writeFileFunc(name, data, perm)
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
			writeFileFunc: func(filename string, data []byte, perm fs.FileMode) error {
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
				writeFileFunc: func(filename string, data []byte, perm fs.FileMode) error {
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
				writeFileFunc: func(filename string, data []byte, perm fs.FileMode) error {
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
			fileWriter.writeFileFunc = func(filename string, data []byte, perm fs.FileMode) error {
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

func TestService_Seed_BatchSuccess(t *testing.T) {
	var client DynamoClientMock
	var service Service
	logger := logging.New(true)
	ctx := logging.WithContext(context.Background(), logger)

	callBatchWrite := 0
	itemsWritten := 0

	client = DynamoClientMock{
		BatchWriteItemsFunc: func(ctx context.Context, tableName string, items []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
			callBatchWrite++
			itemsWritten += len(items)
			return &dynamodb.BatchWriteItemOutput{}, nil
		},
	}

	service = Service{
		client:  &client,
		dryRun:  false,
		logger:  logger,
		emitter: &mockEmitter{},
		fileWriter: &mockWriteFile{
			readFileFunc: func(filename string) ([]byte, error) {
				// Return 50 items
				items := make([]map[string]any, 50)
				for i := 0; i < 50; i++ {
					items[i] = map[string]any{
						"pk": fmt.Sprintf("pk-%d", i),
						"sk": fmt.Sprintf("sk-%d", i),
					}
				}
				return json.Marshal(items)
			},
			writeFileFunc: func(filename string, data []byte, perm fs.FileMode) error {
				return nil
			},
		},
	}

	err := service.Seed(ctx, "my-table", "test.json")
	odize.AssertNoError(t, err)
	odize.AssertTrue(t, callBatchWrite >= 2) // At least 2 batches for 50 items
	odize.AssertEqual(t, 50, itemsWritten)
}

func TestService_Seed_RetryLogic(t *testing.T) {
	var client DynamoClientMock
	var service Service
	logger := logging.New(true)
	ctx := logging.WithContext(context.Background(), logger)

	callCount := 0

	client = DynamoClientMock{
		BatchWriteItemsFunc: func(ctx context.Context, tableName string, items []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
			callCount++
			if callCount == 1 {
				// First call returns unprocessed items
				return &dynamodb.BatchWriteItemOutput{
					UnprocessedItems: map[string][]types.WriteRequest{
						"my-table": {
							{PutRequest: &types.PutRequest{Item: items[0]}},
						},
					},
				}, nil
			}
			// Second call succeeds
			return &dynamodb.BatchWriteItemOutput{}, nil
		},
	}

	service = Service{
		client:  &client,
		dryRun:  false,
		logger:  logger,
		emitter: &mockEmitter{},
		fileWriter: &mockWriteFile{
			readFileFunc: func(filename string) ([]byte, error) {
				items := []map[string]any{
					{"pk": "pk-1", "sk": "sk-1"},
				}
				return json.Marshal(items)
			},
			writeFileFunc: func(filename string, data []byte, perm fs.FileMode) error {
				return nil
			},
		},
	}

	err := service.Seed(ctx, "my-table", "test.json")
	odize.AssertNoError(t, err)
	odize.AssertEqual(t, 2, callCount) // Should retry once
}

func TestService_Seed_FailureFile(t *testing.T) {
	var client DynamoClientMock
	var service Service
	logger := logging.New(true)
	ctx := logging.WithContext(context.Background(), logger)

	failureFilePath := ""
	failureFileData := []byte{}
	tempFileCreated := false

	client = DynamoClientMock{
		BatchWriteItemsFunc: func(ctx context.Context, tableName string, items []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
			// Always return unprocessed items to trigger failure
			unprocessed := make([]types.WriteRequest, len(items))
			for i, item := range items {
				unprocessed[i] = types.WriteRequest{PutRequest: &types.PutRequest{Item: item}}
			}
			return &dynamodb.BatchWriteItemOutput{
				UnprocessedItems: map[string][]types.WriteRequest{
					"my-table": unprocessed,
				},
			}, nil
		},
	}

	service = Service{
		client:  &client,
		dryRun:  false,
		logger:  logger,
		emitter: &mockEmitter{},
		fileWriter: &mockWriteFile{
			readFileFunc: func(filename string) ([]byte, error) {
				items := []map[string]any{
					{"pk": "pk-1", "sk": "sk-1", "data": "value1"},
					{"pk": "pk-2", "sk": "sk-2", "data": "value2"},
				}
				return json.Marshal(items)
			},
			writeFileFunc: func(filename string, data []byte, perm fs.FileMode) error {
				if contains(filename, "failed.") {
					failureFilePath = filename
					failureFileData = data
					// Create temp file to simulate real file write for cleanup testing
					os.WriteFile(filename, data, perm)
					tempFileCreated = true
				}
				return nil
			},
		},
	}

	err := service.Seed(ctx, "my-table", "test.json")

	// Clean up test file if it was created
	defer func() {
		if tempFileCreated && failureFilePath != "" {
			os.Remove(failureFilePath)
		}
	}()

	// Should return error indicating failures
	odize.AssertTrue(t, err != nil)

	// Verify failure file was created with timestamp
	odize.AssertTrue(t, failureFilePath != "")
	odize.AssertTrue(t, contains(failureFilePath, "failed."))
	odize.AssertTrue(t, contains(failureFilePath, ".json"))

	// Verify failure file contains the items
	var failedItems []map[string]any
	json.Unmarshal(failureFileData, &failedItems)
	odize.AssertEqual(t, 2, len(failedItems))

	// Verify temp file was created and will be cleaned up by defer
	if tempFileCreated {
		_, statErr := os.Stat(failureFilePath)
		odize.AssertTrue(t, statErr == nil) // File should exist
	}
}

func TestService_Seed_DryRun(t *testing.T) {
	var client DynamoClientMock
	var service Service
	logger := logging.New(true)
	ctx := logging.WithContext(context.Background(), logger)

	callBatchWrite := 0

	client = DynamoClientMock{
		BatchWriteItemsFunc: func(ctx context.Context, tableName string, items []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
			callBatchWrite++
			return nil, errors.New("should not be called")
		},
	}

	service = Service{
		client:  &client,
		dryRun:  true,
		logger:  logger,
		emitter: &mockEmitter{},
		fileWriter: &mockWriteFile{
			readFileFunc: func(filename string) ([]byte, error) {
				items := []map[string]any{
					{"pk": "pk-1", "sk": "sk-1"},
				}
				return json.Marshal(items)
			},
			writeFileFunc: func(filename string, data []byte, perm fs.FileMode) error {
				return nil
			},
		},
	}

	err := service.Seed(ctx, "my-table", "test.json")
	odize.AssertNoError(t, err)
	odize.AssertEqual(t, 0, callBatchWrite) // Should not call BatchWriteItems
}

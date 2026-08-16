package goety

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/goety/internal/logging"
	"github.com/code-gorilla-au/odize"
)

type mockEmitter struct {
	publishFunc func(message string)
}

func (m *mockEmitter) Publish(string) {}

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

type mockWriter struct {
	writeFunc func(p []byte) (n int, err error)
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	return m.writeFunc(p)
}

func TestService_Dump(t *testing.T) {
	var client DynamoClientMock
	var service Service
	var writer mockWriter
	logger := logging.New(true)
	ctx := logging.WithContext(context.Background(), logger)

	callScanAll := 0

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

		writer = mockWriter{
			writeFunc: func(p []byte) (n int, err error) {
				return len(p), nil
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
	})

	err := group.
		Test("should dump items", func(t *testing.T) {
			err := service.Dump(ctx, "my-table", &writer)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
		}).
		Test("should dump items on dry run", func(t *testing.T) {
			service.dryRun = true

			var buf bytes.Buffer
			err := service.Dump(ctx, "my-table", &buf)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			odize.AssertTrue(t, !bytes.Contains(buf.Bytes(), []byte("pk")))
		}).
		Test("should output", func(t *testing.T) {
			var buf bytes.Buffer
			err := service.Dump(ctx, "my-table", &buf)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			var items []map[string]any
			odize.AssertNoError(t, json.Unmarshal(buf.Bytes(), &items))
			odize.AssertEqual(t, 1, len(items))
			odize.AssertEqual(t, "pk", items[0]["pk"])
			odize.AssertEqual(t, "sk", items[0]["sk"])
		}).
		Test("should output raw", func(t *testing.T) {
			var buf bytes.Buffer
			err := service.Dump(ctx, "my-table", &buf, WithRawOutput(true))
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			var items []map[string]any
			odize.AssertNoError(t, json.Unmarshal(buf.Bytes(), &items))
			odize.AssertEqual(t, 1, len(items))
			odize.AssertEqual(t, "pk", items[0]["pk"].(map[string]any)["S"])
			odize.AssertEqual(t, "sk", items[0]["sk"].(map[string]any)["S"])
		}).
		Test("should output items across multiple scan batches", func(t *testing.T) {
			scanCall := 0
			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				scanCall++
				if scanCall == 1 {
					return &dynamodb.ScanOutput{
						Items: []map[string]types.AttributeValue{
							{
								"pk": &types.AttributeValueMemberS{Value: "pk1"},
								"sk": &types.AttributeValueMemberS{Value: "sk1"},
							},
							{
								"pk": &types.AttributeValueMemberS{Value: "pk2"},
								"sk": &types.AttributeValueMemberS{Value: "sk2"},
							},
						},
						LastEvaluatedKey: map[string]types.AttributeValue{
							"pk": &types.AttributeValueMemberS{Value: "pk2"},
						},
					}, nil
				}
				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{
						{
							"pk": &types.AttributeValueMemberS{Value: "pk3"},
							"sk": &types.AttributeValueMemberS{Value: "sk3"},
						},
					},
				}, nil
			}

			var buf bytes.Buffer
			err := service.Dump(ctx, "my-table", &buf)
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, 2, scanCall)

			var items []map[string]any
			odize.AssertNoError(t, json.Unmarshal(buf.Bytes(), &items))
			odize.AssertEqual(t, 3, len(items))
			odize.AssertEqual(t, "pk1", items[0]["pk"])
			odize.AssertEqual(t, "pk2", items[1]["pk"])
			odize.AssertEqual(t, "pk3", items[2]["pk"])
		}).
		Test("should output empty array when table has no items", func(t *testing.T) {
			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{}, nil
			}

			var buf bytes.Buffer
			err := service.Dump(ctx, "my-table", &buf)
			odize.AssertNoError(t, err)

			var items []map[string]any
			odize.AssertNoError(t, json.Unmarshal(buf.Bytes(), &items))
			odize.AssertEqual(t, 0, len(items))
		}).
		Test("should return error if scan fails", func(t *testing.T) {
			expectedErr := errors.New("scan all error")
			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				return nil, expectedErr
			}

			err := service.Dump(ctx, "my-table", &writer)
			odize.AssertTrue(t, errors.Is(err, expectedErr))
		}).
		Test("should return error if file write fails", func(t *testing.T) {
			expectedErr := errors.New("write file error")
			writer.writeFunc = func(data []byte) (int, error) {
				return 0, expectedErr
			}

			err := service.Dump(ctx, "my-table", &writer)
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

			err := service.Dump(ctx, "my-table", &writer, WithAttrs(attrExp))
			odize.AssertNoError(t, err)
		}).
		Run()

	odize.AssertNoError(t, err)

}

func TestService_Seed(t *testing.T) {
	var client DynamoClientMock
	logger := logging.New(true)
	ctx := logging.WithContext(context.Background(), logger)

	group := odize.NewGroup(t, nil)

	group.BeforeEach(func() {
		client = DynamoClientMock{
			PutFunc: func(ctx context.Context, input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
				return &dynamodb.PutItemOutput{}, nil
			},
		}
	})

	err := group.
		Test("should return error if file is not a json array", func(t *testing.T) {
			service := Service{
				client:  &client,
				logger:  logger,
				emitter: &mockEmitter{},
			}

			err := service.Seed(ctx, "my-table", bytes.NewReader([]byte(`{"a":1}`)))
			odize.AssertTrue(t, err != nil)
		}).
		Test("should return error on invalid json", func(t *testing.T) {
			service := Service{
				client:  &client,
				logger:  logger,
				emitter: &mockEmitter{},
			}

			err := service.Seed(ctx, "my-table", bytes.NewReader([]byte(`not json`)))
			odize.AssertTrue(t, err != nil)
		}).
		Test("should seed items", func(t *testing.T) {
			service := Service{
				client:  &client,
				logger:  logger,
				emitter: &mockEmitter{},
			}

			err := service.Seed(ctx, "my-table", bytes.NewReader([]byte(`[{"pk":"pk","sk":"sk"}]`)))
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, 1, len(client.PutCalls()))
		}).
		Run()

	odize.AssertNoError(t, err)
}

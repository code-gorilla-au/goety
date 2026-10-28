package goety

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/goety/internal/logging"
	"github.com/code-gorilla-au/odize"
)

func TestService_Purge(t *testing.T) {
	var client DynamoClientMock
	var service Service
	logger := logging.New(true)
	ctx := context.Background()

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
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "scan items for purge:"))
		}).
		Test("should return error if batch write fails", func(t *testing.T) {
			expectedErr := errors.New("batch write error")
			client.BatchDeleteItemsFunc = func(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
				return nil, expectedErr
			}

			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertTrue(t, errors.Is(err, expectedErr))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "delete items for purge:"))
		}).
		Test("should not fail if scan has no items", func(t *testing.T) {

			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{}, nil
			}

			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, 0, callBatchDelete)
		}).
		Test("should report progress", func(t *testing.T) {
			var messages []string
			service.progress = func(message string) {
				messages = append(messages, message)
			}

			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 3, len(messages))
			odize.AssertEqual(t, "scanning table my-table for items to purge", messages[0])
			odize.AssertEqual(t, "deleted 1 items", messages[1])
			odize.AssertTrue(t, strings.HasPrefix(messages[2], "purge complete, deleted 1 items, time taken ["))
		}).
		Test("should work without a progress callback", func(t *testing.T) {
			service.progress = nil

			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertNoError(t, err)
		}).
		Run()

	odize.AssertNoError(t, err)

}

type mockFileReadWriter struct {
	createFunc func(filename string, perm fs.FileMode) (io.WriteCloser, error)
	openFunc   func(filename string) (io.ReadCloser, error)
}

func (m *mockFileReadWriter) Create(name string, perm fs.FileMode) (io.WriteCloser, error) {
	return m.createFunc(name, perm)
}

func (m *mockFileReadWriter) Open(name string) (io.ReadCloser, error) {
	return m.openFunc(name)
}

type mockWriteCloser struct {
	bytes.Buffer
	writeErr error
}

func (m *mockWriteCloser) Write(p []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.Buffer.Write(p)
}

func (m *mockWriteCloser) Close() error { return nil }

// captureStdout returns everything fn writes to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	odize.AssertNoError(t, err)

	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	odize.AssertNoError(t, w.Close())
	os.Stdout = orig

	data, err := io.ReadAll(r)
	odize.AssertNoError(t, err)

	return string(data)
}

type mockReadCloser struct {
	*bytes.Buffer
	closeErr error
}

func (m *mockReadCloser) Close() error { return m.closeErr }

func TestService_Dump(t *testing.T) {
	var client DynamoClientMock
	var service Service
	var fileReadWriter mockFileReadWriter
	logger := logging.New(true)
	ctx := context.Background()

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

		fileReadWriter = mockFileReadWriter{
			createFunc: func(filename string, perm fs.FileMode) (io.WriteCloser, error) {
				fmt.Println("creating file", filename)
				callWriteFile++
				return &mockWriteCloser{}, nil
			},
		}

		service = Service{
			client:           &client,
			dryRun:           false,
			logger:           logger,
			fileReaderWriter: &fileReadWriter,
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
			var buf mockWriteCloser
			fileReadWriter = mockFileReadWriter{
				createFunc: func(filename string, perm fs.FileMode) (io.WriteCloser, error) {
					callWriteFile++
					return &buf, nil
				},
			}

			err := service.Dump(ctx, "my-table", "path")
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			odize.AssertEqual(t, 1, callWriteFile)
			odize.AssertEqual(t, "[{\"pk\":\"pk\",\"sk\":\"sk\"}]", buf.String())
		}).
		Test("should output raw", func(t *testing.T) {
			var buf mockWriteCloser
			fileReadWriter = mockFileReadWriter{
				createFunc: func(filename string, perm fs.FileMode) (io.WriteCloser, error) {
					callWriteFile++
					return &buf, nil
				},
			}

			err := service.Dump(ctx, "my-table", "path", WithRawOutput(true))
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callScanAll)
			odize.AssertEqual(t, 1, callWriteFile)
			odize.AssertEqual(t, `[{"pk":{"S":"pk"},"sk":{"S":"sk"}}]`, buf.String())
		}).
		Test("should return error if scan fails", func(t *testing.T) {
			expectedErr := errors.New("scan all error")
			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				return nil, expectedErr
			}

			err := service.Dump(ctx, "my-table", "path")
			odize.AssertTrue(t, errors.Is(err, expectedErr))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "scan items for dump:"))
		}).
		Test("should close the json array when a scan fails mid dump", func(t *testing.T) {
			var buf mockWriteCloser
			fileReadWriter.createFunc = func(filename string, perm fs.FileMode) (io.WriteCloser, error) {
				return &buf, nil
			}

			expectedErr := errors.New("scan page error")
			calls := 0
			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				calls++
				if calls == 2 {
					return nil, expectedErr
				}

				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{
						{
							"pk": &types.AttributeValueMemberS{Value: "pk"},
							"sk": &types.AttributeValueMemberS{Value: "sk"},
						},
					},
					LastEvaluatedKey: map[string]types.AttributeValue{
						"pk": &types.AttributeValueMemberS{Value: "pk"},
					},
				}, nil
			}

			err := service.Dump(ctx, "my-table", "path")
			odize.AssertTrue(t, errors.Is(err, expectedErr))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "scan items for dump:"))
			odize.AssertEqual(t, "[{\"pk\":\"pk\",\"sk\":\"sk\"}]", buf.String())
		}).
		Test("should return error if create file fails", func(t *testing.T) {
			expectedErr := errors.New("write file error")
			fileReadWriter.createFunc = func(filename string, perm fs.FileMode) (io.WriteCloser, error) {
				return nil, expectedErr
			}

			err := service.Dump(ctx, "my-table", "path")
			odize.AssertTrue(t, errors.Is(err, expectedErr))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "create dump file:"))
		}).
		Test("should return error if writing items fails", func(t *testing.T) {
			expectedErr := errors.New("write error")
			fileReadWriter.createFunc = func(filename string, perm fs.FileMode) (io.WriteCloser, error) {
				return &mockWriteCloser{writeErr: expectedErr}, nil
			}

			err := service.Dump(ctx, "my-table", "path")
			odize.AssertTrue(t, errors.Is(err, expectedErr))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "write opening bracket to dump file:"))
		}).
		Test("should pass the limit to the scan input", func(t *testing.T) {
			var gotLimit *int32
			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				gotLimit = input.Limit
				return &dynamodb.ScanOutput{}, nil
			}

			err := service.Dump(ctx, "my-table", "path", WithLimit(10))
			odize.AssertNoError(t, err)

			odize.AssertTrue(t, gotLimit != nil)
			odize.AssertEqual(t, int32(10), *gotLimit)
		}).
		Test("should not set a scan limit by default", func(t *testing.T) {
			var gotLimit *int32
			client.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				gotLimit = input.Limit
				return &dynamodb.ScanOutput{}, nil
			}

			err := service.Dump(ctx, "my-table", "path")
			odize.AssertNoError(t, err)

			odize.AssertTrue(t, gotLimit == nil)
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

func TestService_Seed(t *testing.T) {
	var client DynamoClientMock
	var service Service
	var fileReadWriter mockFileReadWriter
	logger := logging.New(true)
	ctx := context.Background()

	callOpen := 0
	callBatchWrite := 0

	group := odize.NewGroup(t, nil)

	group.BeforeEach(func() {
		callOpen = 0
		callBatchWrite = 0

		client = DynamoClientMock{
			BatchWriteItemsFunc: func(ctx context.Context, tableName string, items []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
				callBatchWrite++
				return &dynamodb.BatchWriteItemOutput{}, nil
			},
		}

		fileReadWriter = mockFileReadWriter{
			openFunc: func(filename string) (io.ReadCloser, error) {
				callOpen++
				return &mockReadCloser{Buffer: bytes.NewBufferString(`[{"pk":"pk","sk":"sk"}]`)}, nil
			},
		}

		service = Service{
			client:           &client,
			dryRun:           false,
			logger:           logger,
			fileReaderWriter: &fileReadWriter,
		}
	})

	group.AfterEach(func() {
		callOpen = 0
		callBatchWrite = 0
	})

	err := group.
		Test("should seed items", func(t *testing.T) {
			err := service.Seed(ctx, "my-table", "path")
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callOpen)
			odize.AssertEqual(t, 1, callBatchWrite)
		}).
		Test("should not batch write on dry run", func(t *testing.T) {
			service.dryRun = true

			err := service.Seed(ctx, "my-table", "path")
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 1, callOpen)
			odize.AssertEqual(t, 0, callBatchWrite)
		}).
		Test("should print dry run items as a single json array", func(t *testing.T) {
			service.dryRun = true
			fileReadWriter.openFunc = func(filename string) (io.ReadCloser, error) {
				return &mockReadCloser{Buffer: bytes.NewBufferString(`[{"pk":"a"},{"pk":"b"}]`)}, nil
			}

			out := captureStdout(t, func() {
				err := service.Seed(ctx, "my-table", "path")
				odize.AssertNoError(t, err)
			})

			var items []map[string]any
			err := json.Unmarshal([]byte(out), &items)
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, 2, len(items))
			odize.AssertEqual(t, "a", items[0]["pk"])
			odize.AssertEqual(t, "b", items[1]["pk"])
		}).
		Test("should return error if open fails", func(t *testing.T) {
			expectedErr := errors.New("open error")
			fileReadWriter.openFunc = func(filename string) (io.ReadCloser, error) {
				return nil, expectedErr
			}

			err := service.Seed(ctx, "my-table", "path")
			odize.AssertTrue(t, errors.Is(err, expectedErr))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "open seed file:"))
		}).
		Test("should return error if decode fails", func(t *testing.T) {
			fileReadWriter.openFunc = func(filename string) (io.ReadCloser, error) {
				return &mockReadCloser{Buffer: bytes.NewBufferString(`not json`)}, nil
			}

			err := service.Seed(ctx, "my-table", "path")
			odize.AssertTrue(t, err != nil)
		}).
		Test("should return error if batch write fails", func(t *testing.T) {
			expectedErr := errors.New("batch write error")
			client.BatchWriteItemsFunc = func(ctx context.Context, tableName string, items []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
				return nil, expectedErr
			}

			err := service.Seed(ctx, "my-table", "path")
			odize.AssertTrue(t, errors.Is(err, expectedErr))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "write seed items:"))
		}).
		Test("should batch items into groups of 25", func(t *testing.T) {
			var sb strings.Builder
			sb.WriteString("[")
			for i := 0; i < 30; i++ {
				if i > 0 {
					sb.WriteString(",")
				}
				fmt.Fprintf(&sb, `{"pk":"item","idx":%d}`, i)
			}
			sb.WriteString("]")

			fileReadWriter.openFunc = func(filename string) (io.ReadCloser, error) {
				return &mockReadCloser{Buffer: bytes.NewBufferString(sb.String())}, nil
			}

			callCount := 0
			client.BatchWriteItemsFunc = func(ctx context.Context, tableName string, items []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
				callCount++
				if callCount == 1 && len(items) != 25 {
					t.Fatalf("expected first batch of 25, got %d", len(items))
				}
				if callCount == 2 && len(items) != 5 {
					t.Fatalf("expected second batch of 5, got %d", len(items))
				}
				return &dynamodb.BatchWriteItemOutput{}, nil
			}

			err := service.Seed(ctx, "my-table", "path")
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, 2, callCount)
		}).
		Run()

	odize.AssertNoError(t, err)
}

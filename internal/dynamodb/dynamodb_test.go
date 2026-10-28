package dynamodb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/goety/internal/logging"
	"github.com/code-gorilla-au/odize"
)

func TestClient_Scan(t *testing.T) {
	logger := logging.New(false)
	ctx := context.Background()
	var client Client
	var db ddbClientMock

	group := odize.NewGroup(t, nil)
	group.BeforeEach(func() {
		db = ddbClientMock{
			ScanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{
						{
							"key": &types.AttributeValueMemberS{Value: "value"},
						},
					},
				}, nil
			},
		}

		client = Client{
			logger: logger,
			db:     &db,
		}
	})

	err := group.
		Test("should scan table", func(t *testing.T) {

			input := dynamodb.ScanInput{}
			result, err := client.Scan(ctx, &input)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, result.Items[0]["key"].(*types.AttributeValueMemberS).Value, "value")
		}).
		Test("should wrap scan errors", func(t *testing.T) {
			expectedErr := errors.ErrUnsupported
			db.ScanFunc = func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
				return nil, expectedErr
			}

			_, err := client.Scan(ctx, &dynamodb.ScanInput{})
			odize.AssertTrue(t, errors.Is(err, expectedErr))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "scan table:"))
		}).
		Run()

	odize.AssertNoError(t, err)
}

func TestClient_BatchDeleteItems(t *testing.T) {
	logger := logging.New(true)
	ctx := context.Background()
	var client Client
	var db ddbClientMock

	batchWrite := 0

	group := odize.NewGroup(t, nil)
	group.BeforeEach(func() {
		db = ddbClientMock{
			BatchWriteItemFunc: func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
				batchWrite++

				if batchWrite == 1 {

					return &dynamodb.BatchWriteItemOutput{
						UnprocessedItems: map[string][]types.WriteRequest{
							"key": {
								{
									DeleteRequest: &types.DeleteRequest{
										Key: map[string]types.AttributeValue{
											"key": &types.AttributeValueMemberS{Value: "value"},
										},
									},
								},
							},
						},
					}, nil
				}

				return &dynamodb.BatchWriteItemOutput{}, nil

			},
		}

		client = Client{
			logger: logger,
			db:     &db,
		}
	})

	group.AfterEach(func() {
		batchWrite = 0
	})

	err := group.
		Test("should not make db call on dry run", func(t *testing.T) {
			client.dryRun = true

			input := []map[string]types.AttributeValue{
				{
					"key": &types.AttributeValueMemberS{Value: "value"},
				},
			}
			_, err := client.BatchDeleteItems(ctx, "table", input)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 0, batchWrite)

		}).
		Test("should run twice for unprocessed", func(t *testing.T) {

			input := []map[string]types.AttributeValue{
				{
					"key": &types.AttributeValueMemberS{Value: "value"},
				},
			}
			_, err := client.BatchDeleteItems(ctx, "table", input)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 2, batchWrite)

		}).
		Test("should return error on db error", func(t *testing.T) {
			db.BatchWriteItemFunc = func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
				return &dynamodb.BatchWriteItemOutput{}, errors.ErrUnsupported
			}

			input := []map[string]types.AttributeValue{
				{
					"key": &types.AttributeValueMemberS{Value: "value"},
				},
			}
			_, err := client.BatchDeleteItems(ctx, "table", input)
			odize.AssertError(t, err)
			odize.AssertTrue(t, errors.Is(err, errors.ErrUnsupported))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "batch delete items:"))

		}).
		Test("should return early if no unprocessed items remain", func(t *testing.T) {
			db.BatchWriteItemFunc = func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
				return &dynamodb.BatchWriteItemOutput{}, nil
			}

			input := []map[string]types.AttributeValue{
				{
					"key": &types.AttributeValueMemberS{Value: "value"},
				},
			}
			_, err := client.BatchDeleteItems(ctx, "table", input)
			odize.AssertNoError(t, err)

		}).
		Run()

	odize.AssertNoError(t, err)
}

func TestClient_BatchWriteItems(t *testing.T) {
	logger := logging.New(true)
	ctx := context.Background()
	var client Client
	var db ddbClientMock

	batchWrite := 0

	group := odize.NewGroup(t, nil)
	group.BeforeEach(func() {
		db = ddbClientMock{
			BatchWriteItemFunc: func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
				batchWrite++

				if batchWrite == 1 {
					return &dynamodb.BatchWriteItemOutput{
						UnprocessedItems: map[string][]types.WriteRequest{
							"key": {
								{
									PutRequest: &types.PutRequest{
										Item: map[string]types.AttributeValue{
											"key": &types.AttributeValueMemberS{Value: "value"},
										},
									},
								},
							},
						},
					}, nil
				}

				return &dynamodb.BatchWriteItemOutput{}, nil
			},
		}

		client = Client{
			logger: logger,
			db:     &db,
		}
	})

	group.AfterEach(func() {
		batchWrite = 0
	})

	err := group.
		Test("should not make db call on dry run", func(t *testing.T) {
			client.dryRun = true

			input := []map[string]types.AttributeValue{
				{
					"key": &types.AttributeValueMemberS{Value: "value"},
				},
			}
			_, err := client.BatchWriteItems(ctx, "table", input)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 0, batchWrite)
		}).
		Test("should run twice for unprocessed", func(t *testing.T) {
			input := []map[string]types.AttributeValue{
				{
					"key": &types.AttributeValueMemberS{Value: "value"},
				},
			}
			_, err := client.BatchWriteItems(ctx, "table", input)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, 2, batchWrite)
		}).
		Test("should return error on db error", func(t *testing.T) {
			db.BatchWriteItemFunc = func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
				return &dynamodb.BatchWriteItemOutput{}, errors.ErrUnsupported
			}

			input := []map[string]types.AttributeValue{
				{
					"key": &types.AttributeValueMemberS{Value: "value"},
				},
			}
			_, err := client.BatchWriteItems(ctx, "table", input)
			odize.AssertError(t, err)
			odize.AssertTrue(t, errors.Is(err, errors.ErrUnsupported))
			odize.AssertTrue(t, strings.HasPrefix(err.Error(), "batch write items:"))
		}).
		Test("should return empty result for empty input", func(t *testing.T) {
			output, err := client.BatchWriteItems(ctx, "table", nil)
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, 0, len(output.UnprocessedItems))
			odize.AssertEqual(t, 0, batchWrite)
		}).
		Run()

	odize.AssertNoError(t, err)
}

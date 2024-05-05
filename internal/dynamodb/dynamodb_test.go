package dynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/goety/internal/logging"
	"github.com/code-gorilla-au/odize"
)

func TestClient_Scan(t *testing.T) {
	logger := logging.New(false)
	ctx := logging.WithContext(context.Background(), logger)
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
		Test("should return error on no records", func(t *testing.T) {
			db.ScanFunc = func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{}, nil
			}

			input := dynamodb.ScanInput{}
			_, err := client.Scan(ctx, &input)
			odize.AssertTrue(t, errors.Is(err, ErrNoItems))
		}).
		Run()

	odize.AssertNoError(t, err)
}


func TestClient_BatchDeleteItems(t *testing.T) {
	logger := logging.New(true)
	ctx := logging.WithContext(context.Background(), logger)
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

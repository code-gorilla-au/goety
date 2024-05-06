package dynamodb

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/odize"
)

type mockDDBScanner struct {
	ScanFunc func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error)
}

func (m *mockDDBScanner) Scan(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
	return m.ScanFunc(ctx, input)
}

func TestScanIterator(t *testing.T) {
	group := odize.NewGroup(t, nil)

	var mockScanner *mockDDBScanner

	callIter := 0

	group.BeforeEach(func() {
		mockScanner = &mockDDBScanner{
			ScanFunc: func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				callIter++
				return &dynamodb.ScanOutput{
					LastEvaluatedKey: map[string]types.AttributeValue{
						"key": &types.AttributeValueMemberS{Value: "value"},
					},
				}, nil
			},
		}
	})

	group.AfterEach(func() {
		callIter = 0
	})

	err := group.
		Test("iterator with additional calls should return next invocation", func(t *testing.T) {
			next := ScanIterator(context.Background(), mockScanner)

			output, err, _ := next(&dynamodb.ScanInput{})
			odize.AssertNoError(t, err)
			odize.AssertFalse(t, output == nil)
			odize.AssertEqual(t, 1, callIter)
		}).
		Test("iterator with additional calls should return done as false", func(t *testing.T) {
			next := ScanIterator(context.Background(), mockScanner)

			_, err, done := next(&dynamodb.ScanInput{})
			odize.AssertNoError(t, err)
			odize.AssertFalse(t, done)
		}).
		Test("iterator with no next calls should return empty invocation", func(t *testing.T) {
			mockScanner.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{
					LastEvaluatedKey: nil,
				}, nil
			}
			next := ScanIterator(context.Background(), mockScanner)

			output, err, _ := next(&dynamodb.ScanInput{})
			odize.AssertNoError(t, err)
			odize.AssertFalse(t, output == nil)
		}).
		Test("iterator with no next calls should return done as true", func(t *testing.T) {
			mockScanner.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{
					LastEvaluatedKey: nil,
				}, nil
			}
			next := ScanIterator(context.Background(), mockScanner)

			_, err, done := next(&dynamodb.ScanInput{})
			odize.AssertNoError(t, err)
			odize.AssertTrue(t, done)
		}).
		Run()
	odize.AssertNoError(t, err)
}

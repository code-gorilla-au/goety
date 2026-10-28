package dynamodb

import (
	"context"
	"errors"
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

func TestScanAll(t *testing.T) {
	group := odize.NewGroup(t, nil)

	var mockScanner *mockDDBScanner
	calls := 0

	pagedScan := func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
		calls++
		if calls < 3 {
			return &dynamodb.ScanOutput{
				LastEvaluatedKey: map[string]types.AttributeValue{
					"key": &types.AttributeValueMemberS{Value: "value"},
				},
			}, nil
		}
		return &dynamodb.ScanOutput{}, nil
	}

	group.BeforeEach(func() {
		calls = 0
		mockScanner = &mockDDBScanner{ScanFunc: pagedScan}
	})

	err := group.
		Test("should yield every page until no last evaluated key", func(t *testing.T) {
			pages := 0
			for out, err := range ScanAll(context.Background(), mockScanner, &dynamodb.ScanInput{}) {
				odize.AssertNoError(t, err)
				odize.AssertFalse(t, out == nil)
				pages++
			}
			odize.AssertEqual(t, 3, pages)
			odize.AssertEqual(t, 3, calls)
		}).
		Test("should not mutate the caller input", func(t *testing.T) {
			input := &dynamodb.ScanInput{}
			for range ScanAll(context.Background(), mockScanner, input) {
			}
			odize.AssertTrue(t, input.ExclusiveStartKey == nil)
		}).
		Test("should yield nil and error once then stop on scan error", func(t *testing.T) {
			mockScanner.ScanFunc = func(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
				calls++
				return nil, errors.ErrUnsupported
			}
			yielded := 0
			for out, err := range ScanAll(context.Background(), mockScanner, &dynamodb.ScanInput{}) {
				yielded++
				odize.AssertTrue(t, out == nil)
				odize.AssertTrue(t, errors.Is(err, errors.ErrUnsupported))
			}
			odize.AssertEqual(t, 1, yielded)
			odize.AssertEqual(t, 1, calls)
		}).
		Test("should stop paginating on early break", func(t *testing.T) {
			for range ScanAll(context.Background(), mockScanner, &dynamodb.ScanInput{}) {
				break
			}
			odize.AssertEqual(t, 1, calls)
		}).
		Run()
	odize.AssertNoError(t, err)
}

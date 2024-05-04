package dynamodb

import (
	"context"

	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func ScanIterator(ctx context.Context, scanner Scanner) func(input *ddb.ScanInput) (*ddb.ScanOutput, error, bool) {
	done := false
	var lastEvaluatedKey map[string]types.AttributeValue

	return func(input *ddb.ScanInput) (*ddb.ScanOutput, error, bool) {
		if done {
			return nil, nil, done
		}

		input.ExclusiveStartKey = lastEvaluatedKey

		output, err := scanner.Scan(ctx, input)
		if err != nil {
			done = true
			return output, err, done
		}

		lastEvaluatedKey = output.LastEvaluatedKey

		if lastEvaluatedKey == nil {
			done = true
		}

		return output, nil, done
	}
}

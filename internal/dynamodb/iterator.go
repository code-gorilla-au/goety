package dynamodb

import (
	"context"
	"iter"

	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// ScanAll - iterates over every page of a DynamoDB scan. The caller's input is
// not mutated. On a scan error it yields (nil, err) once and stops. Breaking out
// of the range loop stops pagination.
//
// Example:
//
//	for out, err := range dynamodb.ScanAll(ctx, scanner, input) {
//	    if err != nil { return err }
//	}
func ScanAll(ctx context.Context, scanner Scanner, input *ddb.ScanInput) iter.Seq2[*ddb.ScanOutput, error] {
	return func(yield func(*ddb.ScanOutput, error) bool) {
		in := *input

		for {
			out, err := scanner.Scan(ctx, &in)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(out, nil) {
				return
			}

			if out.LastEvaluatedKey == nil {
				return
			}

			in.ExclusiveStartKey = out.LastEvaluatedKey
		}
	}
}

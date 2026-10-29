package goety

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func WithDumpOptions(opts []DumpOption) *DumpOptions {
	dumpOpts := &DumpOptions{}

	for _, opt := range opts {
		dumpOpts = opt(dumpOpts)
	}

	return dumpOpts
}

// WithAttrs - provide a list of dynamodb attributes the query will return
func WithAttrs(attrs []string) DumpOption {
	return func(opts *DumpOptions) *DumpOptions {
		if len(attrs) == 0 {
			return opts
		}

		opts.ProjectedExpressions = aws.String(strings.Join(attrs, ", "))
		return opts
	}
}

// WithFilterExpression - provide a filter condition for the query
func WithFilterExpression(condition string) DumpOption {
	return func(opts *DumpOptions) *DumpOptions {
		if condition == "" {
			return opts
		}

		opts.FilterExpression = aws.String(condition)
		return opts
	}
}

// WithFilterNameAttrs - provide a map of expression attribute names to filter on
func WithFilterNameAttrs(attrNames map[string]string) DumpOption {
	return func(opts *DumpOptions) *DumpOptions {
		if len(attrNames) == 0 {
			return opts
		}

		opts.FilterNameAttributes = attrNames
		return opts
	}
}

// WithFilterNameValues - provide a map of expression attribute values to filter on
func WithFilterNameValues(attrValues map[string]types.AttributeValue) DumpOption {
	return func(opts *DumpOptions) *DumpOptions {
		if len(attrValues) == 0 {
			return opts
		}

		opts.FilterNameValues = attrValues
		return opts
	}
}

// WithLimit - provide a limit to the query
func WithLimit(limit int32) DumpOption {
	return func(opts *DumpOptions) *DumpOptions {
		if limit == 0 {
			return opts
		}

		opts.Limit = aws.Int32(limit)
		return opts
	}
}

func WithRawOutput(raw bool) DumpOption {
	return func(opts *DumpOptions) *DumpOptions {
		opts.RawOutput = raw
		return opts
	}
}

package goety

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
)


func WithQueryOptions(opts []QueryFuncOpts) *QueryOpts {
	queryOpts := &QueryOpts{}

	for _, opt := range opts {
		queryOpts = opt(queryOpts)
	}

	return queryOpts
}

// WithAttrs - provide a list of dynamodb attributes the query will return
func WithAttrs(attrs []string) QueryFuncOpts {
	return func(opts *QueryOpts) *QueryOpts {
		if len(attrs) == 0 {
			return opts
		}

		opts.ProjectedExpressions = aws.String(strings.Join(attrs, ", "))
		return opts
	}
}

// WithFilterExpression - provide a filter condition for the query
func WithFilterExpression(condition string) QueryFuncOpts {
	return func(opts *QueryOpts) *QueryOpts {
		if condition == "" {
			return opts
		}
		
		opts.FilterCondition = aws.String(condition)
		return opts
	}
}

func WithLimit(limit int32) QueryFuncOpts {
	return func(opts *QueryOpts) *QueryOpts {
		if limit == 0 {
			return opts
		}

		opts.Limit = aws.Int32(limit)
		return opts
	}
}
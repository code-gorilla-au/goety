package goety

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

		opts.FilterExpression = aws.String(condition)
		return opts
	}
}

// WithFilterNameAttrs - provide a list of attribute names to filter on
func WithFilterNameAttrs(attrName string) QueryFuncOpts {
	return func(opts *QueryOpts) *QueryOpts {
		if attrName == "" {
			return opts
		}

		filterNameAttr := make(map[string]string)
		expList := strings.Split(attrName, ",")

		for _, exp := range expList {
			ex := strings.Split(exp, "=")
			if len(ex) < 2 {
				continue
			}
			tKey := strings.TrimSpace(ex[0])
			tVal := strings.TrimSpace(ex[1])
			filterNameAttr[tKey] = tVal
		}

		opts.FilterNameAttributes = filterNameAttr
		return opts
	}
}

// WithFilterNameValues - provide a list of attribute values to filter on
func WithFilterNameValues(attrValues string) QueryFuncOpts {
	return func(opts *QueryOpts) *QueryOpts {
		if attrValues == "" {
			return opts
		}

		filterNameValues := make(map[string]types.AttributeValue)
		expList := strings.Split(attrValues, ",")

		for _, exp := range expList {
			ex := strings.Split(exp, "=")
			if len(ex) < 2 {
				continue
			}
			tKey := strings.TrimSpace(ex[0])
			tVal := types.AttributeValueMemberS{Value: strings.TrimSpace(ex[1])}
			filterNameValues[tKey] = &tVal
		}

		opts.FilterNameValues = filterNameValues
		return opts
	}
}

// WithLimit - provide a limit to the query
func WithLimit(limit int32) QueryFuncOpts {
	return func(opts *QueryOpts) *QueryOpts {
		if limit == 0 {
			return opts
		}

		opts.Limit = aws.Int32(limit)
		return opts
	}
}

func WithRawOutput(raw bool) QueryFuncOpts {
	return func(opts *QueryOpts) *QueryOpts {
		opts.RawOutput = raw
		return opts
	}
}

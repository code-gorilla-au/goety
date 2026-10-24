package goety

import (
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/goety/internal/emitter"
)

type Service struct {
	logger           *slog.Logger
	dryRun           bool
	client           DynamoClient
	fileReaderWriter FileReadWriter
	emitter          emitter.MessagePublisher
}

type TableKeys struct {
	PartitionKey string
	SortKey      string
}

type QueryOpts struct {
	Limit                *int32
	FilterExpression     *string
	ProjectedExpressions *string
	FilterNameAttributes map[string]string
	FilterNameValues     map[string]types.AttributeValue
	RawOutput            bool
}

type QueryFuncOpts = func(*QueryOpts) *QueryOpts

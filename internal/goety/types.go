package goety

import (
	"github.com/code-gorilla-au/goety/internal/emitter"
	"github.com/code-gorilla-au/goety/internal/logging"
)

type Service struct {
	logger     logging.Logger
	dryRun     bool
	client     DynamoClient
	fileWriter fileWriter
	emitter    emitter.MessagePublisher
}

type TableKeys struct {
	PartitionKey string
	SortKey      string
}

type WriteFile struct {
}

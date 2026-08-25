package goety

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/avast/retry-go/v4"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
	ddb "github.com/code-gorilla-au/goety/internal/dynamodb"
	"github.com/code-gorilla-au/goety/internal/emitter"
)

const (
	defaultBatchSize     = 25
	defaultWorkerCount   = 10
	defaultMaxRetries    = 5
	baseRetryDelay       = 100 * time.Millisecond
)

func New(client DynamoClient, logger *slog.Logger, emitter emitter.MessagePublisher, dryRun bool) Service {
	return Service{
		client:     client,
		dryRun:     dryRun,
		logger:     logger,
		fileWriter: &WriteFile{},
		emitter:    emitter,
	}
}

// Purge all items from the given table
//
// Example:
//
//	Purge(ctx, "my-table", TableKeys{ PartitionKey: "pk", SortKey: "sk" })
func (s Service) Purge(ctx context.Context, tableName string, keys TableKeys) error {
	s.emitter.Publish(fmt.Sprintf("scanning table %s for items to purge", tableName))
	now := time.Now()

	done := false
	var err error
	var out *dynamodb.ScanOutput
	next := ddb.ScanIterator(ctx, s.client)

	deleted := 0

	for !done {
		out, err, done = next(&dynamodb.ScanInput{
			TableName:       &tableName,
			AttributesToGet: []string{keys.PartitionKey, keys.SortKey},
			Limit:           aws.Int32(defaultBatchSize),
		})
		if err != nil {
			s.logger.Error("could not scan table", "error", err)
			return err
		}

		if out == nil {
			break
		}

		if len(out.Items) == 0 {
			break
		}

		if s.dryRun {
			s.logger.Debug("dry run enabled")
			prettyPrint(out.Items)
			return nil
		}

		_, err = s.client.BatchDeleteItems(ctx, tableName, out.Items)
		if err != nil {
			s.logger.Error("could not batch delete items", "error", err)
			return err
		}
		deleted += len(out.Items)

		s.emitter.Publish(fmt.Sprintf("deleted %d items", deleted))

	}

	since := time.Since(now)

	s.emitter.PublishBlocking(fmt.Sprintf("purge complete, deleted %d items, time taken [%v]", deleted, since))
	return nil
}

// Dump all items from the given table. Optionally specify a list of attributes to extract.
//
// Example:
//
//	Dump(ctx, "my-table", "path/to/file.json", []string{"attr1", "attr2"})
func (s Service) Dump(ctx context.Context, tableName string, path string, opts ...QueryFuncOpts) error {
	s.emitter.Publish(fmt.Sprintf("dumping table %s to file %s", tableName, path))

	queryOpts := WithQueryOptions(opts)

	done := false
	var err error
	var output *dynamodb.ScanOutput
	next := ddb.ScanIterator(ctx, s.client)

	result := []map[string]any{}

	itemsScanned := 0

	for !done {
		output, err, done = next(
			&dynamodb.ScanInput{
				TableName:                 &tableName,
				ProjectionExpression:      queryOpts.ProjectedExpressions,
				FilterExpression:          queryOpts.FilterExpression,
				ExpressionAttributeNames:  queryOpts.FilterNameAttributes,
				ExpressionAttributeValues: queryOpts.FilterNameValues,
			})
		if err != nil && !errors.Is(err, ddb.ErrNoItems) {
			s.logger.Error("could not scan table", "error", err)
			return err
		}

		if output == nil {
			break
		}

		items, err := transformDumpOutput(output.Items, queryOpts.RawOutput)
		if err != nil {
			s.logger.Error("could not transform items", "error", err)
			return err
		}

		result = append(result, items...)

		itemsScanned += len(items)
		s.emitter.Publish(fmt.Sprintf("scanned %d items", itemsScanned))

	}

	s.emitter.Publish(fmt.Sprintf("scanned %d items", len(result)))

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(result)
		return nil
	}

	message := fmt.Sprintf("saving %d items to file ", len(result)) + path
	s.emitter.Publish(message)
	data, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		s.logger.Error("could not marshal items", "error", marshalErr)
		return marshalErr
	}

	if fileErr := s.fileWriter.WriteFile(path, data, 0644); fileErr != nil {
		s.logger.Error("could not write file", "error", fileErr)
		return fileErr
	}

	s.emitter.Publish("dump complete")
	s.logger.Info("dump complete", "items", itemsScanned)
	return nil
}

// batchResult holds the result of processing a single batch
type batchResult struct {
	processedCount    int
	permanentFailures []map[string]any
}

// retryBatchWithBackoff attempts to write a batch with exponential backoff retry using avast/retry-go
func (s Service) retryBatchWithBackoff(ctx context.Context, tableName string, items []map[string]types.AttributeValue) batchResult {
	result := batchResult{
		processedCount:    0,
		permanentFailures: make([]map[string]any, 0),
	}

	// Convert items to map[string]any format for potential failure file
	originalItems := make([]map[string]any, len(items))
	for i, item := range items {
		flattened, err := ddb.FlattenAttrMap(item)
		if err != nil {
			s.logger.Error("could not flatten item for failure tracking", "error", err)
			originalItems[i] = map[string]any{"error": "could not flatten"}
		} else {
			originalItems[i] = flattened
		}
	}

	currentItems := items

	// Retry configuration with avast/retry-go
	retryOpts := []retry.Option{
		retry.Attempts(uint(defaultMaxRetries)),
		retry.Delay(baseRetryDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(n uint, err error) {
			s.logger.Debug("retrying batch write", "attempt", n+1, "error", err, "itemCount", len(currentItems))
		}),
		retry.RetryIf(func(err error) bool {
			return isRetryableError(err)
		}),
	}

	var processedInSuccess int

	// Execute with retry
	err := retry.Do(
		func() error {
			output, err := s.client.BatchWriteItems(ctx, tableName, currentItems)
			if err != nil {
				return err
			}

			// Process successful attempt
			if output.UnprocessedItems != nil && len(output.UnprocessedItems[tableName]) > 0 {
				// Some items were not processed - extract them for next retry
				unprocessed := output.UnprocessedItems[tableName]
				processedInSuccess += len(currentItems) - len(unprocessed)

				// Convert unprocessed WriteRequests back to AttributeValue maps
				currentItems = make([]map[string]types.AttributeValue, 0, len(unprocessed))
				for _, req := range unprocessed {
					if req.PutRequest != nil {
						currentItems = append(currentItems, req.PutRequest.Item)
					}
				}

				// Return error to trigger retry for unprocessed items
				return fmt.Errorf("%d items unprocessed", len(currentItems))
			}

			// All items processed successfully
			processedInSuccess += len(currentItems)
			currentItems = nil // Clear currentItems to indicate success
			return nil
		},
		retryOpts...,
	)

	result.processedCount = processedInSuccess

	// If retry.Do returned an error, it means all retries were exhausted
	// Any remaining items in currentItems are permanent failures
	if err != nil {
		for _, item := range currentItems {
			flattened, flattenErr := ddb.FlattenAttrMap(item)
			if flattenErr != nil {
				result.permanentFailures = append(result.permanentFailures, map[string]any{"error": "could not flatten failed item"})
			} else {
				result.permanentFailures = append(result.permanentFailures, flattened)
			}
		}
	}

	return result
}

// isRetryableError determines if a DynamoDB error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Unprocessed items are retryable
	if strings.Contains(err.Error(), "items unprocessed") {
		return true
	}

	// Check for known retryable error types
	var throttleErr *types.ProvisionedThroughputExceededException
	if errors.As(err, &throttleErr) {
		return true
	}

	var throttlingErr *types.ThrottlingException
	if errors.As(err, &throttlingErr) {
		return true
	}

	// Check error message for common retryable conditions
	errStr := err.Error()
	retryablePatterns := []string{
		"ThrottlingException",
		"ProvisionedThroughputExceededException",
		"ServiceUnavailable",
		"InternalServerError",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// generateFailureFilePath generates a timestamped failure file path
func generateFailureFilePath() string {
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	return fmt.Sprintf("failed.%s.json", timestamp)
}

// Seed a table with items from a json file
func (s Service) Seed(ctx context.Context, tableName string, filePath string) error {
	s.emitter.Publish(fmt.Sprintf("seeding table %s from file %s", tableName, filePath))
	startTime := time.Now()

	data, err := s.fileWriter.ReadFile(filePath)
	if err != nil {
		s.logger.Error("could not read file", "error", err)
		return err
	}

	itemList := []map[string]any{}
	if err := json.Unmarshal(data, &itemList); err != nil {
		s.logger.Error("could not unmarshal file", "error", err)
		return err
	}

	s.emitter.Publish(fmt.Sprintf("%d items to be loaded into table %s", len(itemList), tableName))

	if s.dryRun {
		s.logger.Debug("dry run enabled")
		prettyPrint(itemList)
		return nil
	}

	// Marshal all items to DynamoDB attribute values
	marshaledItems := make([]map[string]types.AttributeValue, 0, len(itemList))
	for i, item := range itemList {
		payload, err := attributevalue.MarshalMap(item)
		if err != nil {
			s.logger.Error("could not marshal item", "error", err, "index", i)
			return err
		}
		marshaledItems = append(marshaledItems, payload)
	}

	// Split into batches of 25 using lo.Chunk
	batches := lo.Chunk(marshaledItems, defaultBatchSize)
	s.logger.Debug("split items into batches", "batchCount", len(batches))

	// Process batches concurrently using worker pool
	resultPool := pool.New().WithContext(ctx).WithMaxGoroutines(defaultWorkerCount)

	var mu struct {
		sync.Mutex
		processedCount int
		failures       []map[string]any
	}

	for batchIdx, batch := range batches {
		batchIndex := batchIdx
		batchCopy := batch

		resultPool.Go(func(ctx context.Context) error {
			s.logger.Debug("processing batch", "batchIndex", batchIndex, "itemCount", len(batchCopy))

			result := s.retryBatchWithBackoff(ctx, tableName, batchCopy)

			mu.Lock()
			mu.processedCount += result.processedCount
			mu.failures = append(mu.failures, result.permanentFailures...)
			mu.Unlock()

			// Emit progress every 10 batches
			if batchIndex%10 == 0 {
				mu.Lock()
				processed := mu.processedCount
				mu.Unlock()
				s.emitter.Publish(fmt.Sprintf("processed %d items", processed))
			}

			return nil
		})
	}

	// Wait for all batches to complete
	if err := resultPool.Wait(); err != nil {
		s.logger.Error("error during batch processing", "error", err)
		return err
	}

	totalProcessed := mu.processedCount
	allPermanentFailures := mu.failures
	duration := time.Since(startTime)

	// Save failures to file if any
	if len(allPermanentFailures) > 0 {
		failureFilePath := generateFailureFilePath()
		s.logger.Warn("some items failed to seed", "failureCount", len(allPermanentFailures), "failureFile", failureFilePath)

		failureData, err := json.Marshal(allPermanentFailures)
		if err != nil {
			s.logger.Error("could not marshal failures", "error", err)
			return fmt.Errorf("seed partially failed: %d items could not be seeded, and failed to create failure file: %w", len(allPermanentFailures), err)
		}

		if err := s.fileWriter.WriteFile(failureFilePath, failureData, 0644); err != nil {
			s.logger.Error("could not write failure file", "error", err, "path", failureFilePath)
			return fmt.Errorf("seed partially failed: %d items could not be seeded, and failed to write failure file: %w", len(allPermanentFailures), err)
		}

		s.emitter.PublishBlocking(fmt.Sprintf("seed complete with %d items processed, %d items failed (saved to %s), time taken [%v]",
			totalProcessed, len(allPermanentFailures), failureFilePath, duration))

		return fmt.Errorf("seed completed with %d failures, see %s for retry", len(allPermanentFailures), failureFilePath)
	}

	s.emitter.PublishBlocking(fmt.Sprintf("seed complete with %d items inserted, time taken [%v]", totalProcessed, duration))
	s.logger.Info("seed complete", "items", totalProcessed, "duration", duration)
	return nil
}

// prettyPrint - prints a pretty json representation of the given value
func prettyPrint(v any) {
	data, err := json.MarshalIndent(v, "\n", "  ")
	if err != nil {
		return
	}

	fmt.Println(string(data))
}

func transformDumpOutput(attrData []map[string]types.AttributeValue, rawOutput bool) ([]map[string]any, error) {
	out := []map[string]any{}

	if !rawOutput {
		items, transformErr := ddb.FlattenAttrList(attrData)
		if transformErr != nil {
			return out, transformErr
		}
		out = append(out, items...)
		return out, nil
	}

	items, err := ddb.ConvertAVValues(attrData)
	if err != nil {
		return out, err
	}

	data, err := json.Marshal(items)
	if err != nil {
		return out, err
	}

	err = json.Unmarshal(data, &out)
	if err != nil {
		return out, err
	}

	return out, nil
}

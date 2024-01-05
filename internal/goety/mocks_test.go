// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package goety

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"sync"
)

// Ensure, that DynamoClientMock does implement DynamoClient.
// If this is not the case, regenerate this file with moq.
var _ DynamoClient = &DynamoClientMock{}

// DynamoClientMock is a mock implementation of DynamoClient.
//
//	func TestSomethingThatUsesDynamoClient(t *testing.T) {
//
//		// make and configure a mocked DynamoClient
//		mockedDynamoClient := &DynamoClientMock{
//			BatchDeleteItemsFunc: func(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
//				panic("mock out the BatchDeleteItems method")
//			},
//			ScanAllFunc: func(ctx context.Context, input *dynamodb.ScanInput) ([]map[string]types.AttributeValue, error) {
//				panic("mock out the ScanAll method")
//			},
//		}
//
//		// use mockedDynamoClient in code that requires DynamoClient
//		// and then make assertions.
//
//	}
type DynamoClientMock struct {
	// BatchDeleteItemsFunc mocks the BatchDeleteItems method.
	BatchDeleteItemsFunc func(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error)

	// ScanAllFunc mocks the ScanAll method.
	ScanAllFunc func(ctx context.Context, input *dynamodb.ScanInput) ([]map[string]types.AttributeValue, error)

	// calls tracks calls to the methods.
	calls struct {
		// BatchDeleteItems holds details about calls to the BatchDeleteItems method.
		BatchDeleteItems []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// TableName is the tableName argument value.
			TableName string
			// Keys is the keys argument value.
			Keys []map[string]types.AttributeValue
		}
		// ScanAll holds details about calls to the ScanAll method.
		ScanAll []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Input is the input argument value.
			Input *dynamodb.ScanInput
		}
	}
	lockBatchDeleteItems sync.RWMutex
	lockScanAll          sync.RWMutex
}

// BatchDeleteItems calls BatchDeleteItemsFunc.
func (mock *DynamoClientMock) BatchDeleteItems(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) (*dynamodb.BatchWriteItemOutput, error) {
	callInfo := struct {
		Ctx       context.Context
		TableName string
		Keys      []map[string]types.AttributeValue
	}{
		Ctx:       ctx,
		TableName: tableName,
		Keys:      keys,
	}
	mock.lockBatchDeleteItems.Lock()
	mock.calls.BatchDeleteItems = append(mock.calls.BatchDeleteItems, callInfo)
	mock.lockBatchDeleteItems.Unlock()
	if mock.BatchDeleteItemsFunc == nil {
		var (
			batchWriteItemOutputOut *dynamodb.BatchWriteItemOutput
			errOut                  error
		)
		return batchWriteItemOutputOut, errOut
	}
	return mock.BatchDeleteItemsFunc(ctx, tableName, keys)
}

// BatchDeleteItemsCalls gets all the calls that were made to BatchDeleteItems.
// Check the length with:
//
//	len(mockedDynamoClient.BatchDeleteItemsCalls())
func (mock *DynamoClientMock) BatchDeleteItemsCalls() []struct {
	Ctx       context.Context
	TableName string
	Keys      []map[string]types.AttributeValue
} {
	var calls []struct {
		Ctx       context.Context
		TableName string
		Keys      []map[string]types.AttributeValue
	}
	mock.lockBatchDeleteItems.RLock()
	calls = mock.calls.BatchDeleteItems
	mock.lockBatchDeleteItems.RUnlock()
	return calls
}

// ScanAll calls ScanAllFunc.
func (mock *DynamoClientMock) ScanAll(ctx context.Context, input *dynamodb.ScanInput) ([]map[string]types.AttributeValue, error) {
	callInfo := struct {
		Ctx   context.Context
		Input *dynamodb.ScanInput
	}{
		Ctx:   ctx,
		Input: input,
	}
	mock.lockScanAll.Lock()
	mock.calls.ScanAll = append(mock.calls.ScanAll, callInfo)
	mock.lockScanAll.Unlock()
	if mock.ScanAllFunc == nil {
		var (
			stringToAttributeValuesOut []map[string]types.AttributeValue
			errOut                     error
		)
		return stringToAttributeValuesOut, errOut
	}
	return mock.ScanAllFunc(ctx, input)
}

// ScanAllCalls gets all the calls that were made to ScanAll.
// Check the length with:
//
//	len(mockedDynamoClient.ScanAllCalls())
func (mock *DynamoClientMock) ScanAllCalls() []struct {
	Ctx   context.Context
	Input *dynamodb.ScanInput
} {
	var calls []struct {
		Ctx   context.Context
		Input *dynamodb.ScanInput
	}
	mock.lockScanAll.RLock()
	calls = mock.calls.ScanAll
	mock.lockScanAll.RUnlock()
	return calls
}
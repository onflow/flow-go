// Code generated by mockery v2.13.1. DO NOT EDIT.

package mock

import (
	context "context"

	cadence "github.com/onflow/cadence"

	flow "github.com/onflow/flow-go-sdk"

	mock "github.com/stretchr/testify/mock"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *Client) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ExecuteScriptAtBlockHeight provides a mock function with given fields: ctx, height, script, arguments
func (_m *Client) ExecuteScriptAtBlockHeight(ctx context.Context, height uint64, script []byte, arguments []cadence.Value) (cadence.Value, error) {
	ret := _m.Called(ctx, height, script, arguments)

	var r0 cadence.Value
	if rf, ok := ret.Get(0).(func(context.Context, uint64, []byte, []cadence.Value) cadence.Value); ok {
		r0 = rf(ctx, height, script, arguments)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cadence.Value)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uint64, []byte, []cadence.Value) error); ok {
		r1 = rf(ctx, height, script, arguments)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteScriptAtBlockID provides a mock function with given fields: ctx, blockID, script, arguments
func (_m *Client) ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments []cadence.Value) (cadence.Value, error) {
	ret := _m.Called(ctx, blockID, script, arguments)

	var r0 cadence.Value
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, []byte, []cadence.Value) cadence.Value); ok {
		r0 = rf(ctx, blockID, script, arguments)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cadence.Value)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier, []byte, []cadence.Value) error); ok {
		r1 = rf(ctx, blockID, script, arguments)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteScriptAtLatestBlock provides a mock function with given fields: ctx, script, arguments
func (_m *Client) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments []cadence.Value) (cadence.Value, error) {
	ret := _m.Called(ctx, script, arguments)

	var r0 cadence.Value
	if rf, ok := ret.Get(0).(func(context.Context, []byte, []cadence.Value) cadence.Value); ok {
		r0 = rf(ctx, script, arguments)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cadence.Value)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []byte, []cadence.Value) error); ok {
		r1 = rf(ctx, script, arguments)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccount provides a mock function with given fields: ctx, address
func (_m *Client) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	ret := _m.Called(ctx, address)

	var r0 *flow.Account
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address) *flow.Account); ok {
		r0 = rf(ctx, address)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Account)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Address) error); ok {
		r1 = rf(ctx, address)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccountAtBlockHeight provides a mock function with given fields: ctx, address, blockHeight
func (_m *Client) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, blockHeight uint64) (*flow.Account, error) {
	ret := _m.Called(ctx, address, blockHeight)

	var r0 *flow.Account
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address, uint64) *flow.Account); ok {
		r0 = rf(ctx, address, blockHeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Account)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Address, uint64) error); ok {
		r1 = rf(ctx, address, blockHeight)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccountAtLatestBlock provides a mock function with given fields: ctx, address
func (_m *Client) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	ret := _m.Called(ctx, address)

	var r0 *flow.Account
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address) *flow.Account); ok {
		r0 = rf(ctx, address)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Account)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Address) error); ok {
		r1 = rf(ctx, address)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockByHeight provides a mock function with given fields: ctx, height
func (_m *Client) GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, error) {
	ret := _m.Called(ctx, height)

	var r0 *flow.Block
	if rf, ok := ret.Get(0).(func(context.Context, uint64) *flow.Block); ok {
		r0 = rf(ctx, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Block)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uint64) error); ok {
		r1 = rf(ctx, height)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockByID provides a mock function with given fields: ctx, blockID
func (_m *Client) GetBlockByID(ctx context.Context, blockID flow.Identifier) (*flow.Block, error) {
	ret := _m.Called(ctx, blockID)

	var r0 *flow.Block
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.Block); ok {
		r0 = rf(ctx, blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Block)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockHeaderByHeight provides a mock function with given fields: ctx, height
func (_m *Client) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.BlockHeader, error) {
	ret := _m.Called(ctx, height)

	var r0 *flow.BlockHeader
	if rf, ok := ret.Get(0).(func(context.Context, uint64) *flow.BlockHeader); ok {
		r0 = rf(ctx, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.BlockHeader)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uint64) error); ok {
		r1 = rf(ctx, height)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockHeaderByID provides a mock function with given fields: ctx, blockID
func (_m *Client) GetBlockHeaderByID(ctx context.Context, blockID flow.Identifier) (*flow.BlockHeader, error) {
	ret := _m.Called(ctx, blockID)

	var r0 *flow.BlockHeader
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.BlockHeader); ok {
		r0 = rf(ctx, blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.BlockHeader)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCollection provides a mock function with given fields: ctx, colID
func (_m *Client) GetCollection(ctx context.Context, colID flow.Identifier) (*flow.Collection, error) {
	ret := _m.Called(ctx, colID)

	var r0 *flow.Collection
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.Collection); ok {
		r0 = rf(ctx, colID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Collection)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, colID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEventsForBlockIDs provides a mock function with given fields: ctx, eventType, blockIDs
func (_m *Client) GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []flow.Identifier) ([]flow.BlockEvents, error) {
	ret := _m.Called(ctx, eventType, blockIDs)

	var r0 []flow.BlockEvents
	if rf, ok := ret.Get(0).(func(context.Context, string, []flow.Identifier) []flow.BlockEvents); ok {
		r0 = rf(ctx, eventType, blockIDs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.BlockEvents)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, []flow.Identifier) error); ok {
		r1 = rf(ctx, eventType, blockIDs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEventsForHeightRange provides a mock function with given fields: ctx, eventType, startHeight, endHeight
func (_m *Client) GetEventsForHeightRange(ctx context.Context, eventType string, startHeight uint64, endHeight uint64) ([]flow.BlockEvents, error) {
	ret := _m.Called(ctx, eventType, startHeight, endHeight)

	var r0 []flow.BlockEvents
	if rf, ok := ret.Get(0).(func(context.Context, string, uint64, uint64) []flow.BlockEvents); ok {
		r0 = rf(ctx, eventType, startHeight, endHeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.BlockEvents)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, uint64, uint64) error); ok {
		r1 = rf(ctx, eventType, startHeight, endHeight)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutionResultForBlockID provides a mock function with given fields: ctx, blockID
func (_m *Client) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	ret := _m.Called(ctx, blockID)

	var r0 *flow.ExecutionResult
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.ExecutionResult); ok {
		r0 = rf(ctx, blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ExecutionResult)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestBlock provides a mock function with given fields: ctx, isSealed
func (_m *Client) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, error) {
	ret := _m.Called(ctx, isSealed)

	var r0 *flow.Block
	if rf, ok := ret.Get(0).(func(context.Context, bool) *flow.Block); ok {
		r0 = rf(ctx, isSealed)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Block)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, bool) error); ok {
		r1 = rf(ctx, isSealed)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestBlockHeader provides a mock function with given fields: ctx, isSealed
func (_m *Client) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.BlockHeader, error) {
	ret := _m.Called(ctx, isSealed)

	var r0 *flow.BlockHeader
	if rf, ok := ret.Get(0).(func(context.Context, bool) *flow.BlockHeader); ok {
		r0 = rf(ctx, isSealed)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.BlockHeader)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, bool) error); ok {
		r1 = rf(ctx, isSealed)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestProtocolStateSnapshot provides a mock function with given fields: ctx
func (_m *Client) GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error) {
	ret := _m.Called(ctx)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(context.Context) []byte); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransaction provides a mock function with given fields: ctx, txID
func (_m *Client) GetTransaction(ctx context.Context, txID flow.Identifier) (*flow.Transaction, error) {
	ret := _m.Called(ctx, txID)

	var r0 *flow.Transaction
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.Transaction); ok {
		r0 = rf(ctx, txID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Transaction)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, txID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionResult provides a mock function with given fields: ctx, txID
func (_m *Client) GetTransactionResult(ctx context.Context, txID flow.Identifier) (*flow.TransactionResult, error) {
	ret := _m.Called(ctx, txID)

	var r0 *flow.TransactionResult
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.TransactionResult); ok {
		r0 = rf(ctx, txID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.TransactionResult)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, txID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ping provides a mock function with given fields: ctx
func (_m *Client) Ping(ctx context.Context) error {
	ret := _m.Called(ctx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendTransaction provides a mock function with given fields: ctx, tx
func (_m *Client) SendTransaction(ctx context.Context, tx flow.Transaction) error {
	ret := _m.Called(ctx, tx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Transaction) error); ok {
		r0 = rf(ctx, tx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewClient creates a new instance of Client. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewClient(t mockConstructorTestingTNewClient) *Client {
	mock := &Client{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

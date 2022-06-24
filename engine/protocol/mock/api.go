// Code generated by mockery v2.12.1. DO NOT EDIT.

package mock

import (
	context "context"

	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	testing "testing"
)

// API is an autogenerated mock type for the API type
type API struct {
	mock.Mock
}

// GetBlockByHeight provides a mock function with given fields: ctx, height
func (_m *API) GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, error) {
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

// GetBlockByID provides a mock function with given fields: ctx, id
func (_m *API) GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, error) {
	ret := _m.Called(ctx, id)

	var r0 *flow.Block
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.Block); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Block)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockHeaderByHeight provides a mock function with given fields: ctx, height
func (_m *API) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, error) {
	ret := _m.Called(ctx, height)

	var r0 *flow.Header
	if rf, ok := ret.Get(0).(func(context.Context, uint64) *flow.Header); ok {
		r0 = rf(ctx, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Header)
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

// GetBlockHeaderByID provides a mock function with given fields: ctx, id
func (_m *API) GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, error) {
	ret := _m.Called(ctx, id)

	var r0 *flow.Header
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.Header); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Header)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutionResultByBlockID provides a mock function with given fields: ctx, blockID
func (_m *API) GetExecutionResultByBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
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

// GetExecutionResultByID provides a mock function with given fields: ctx, id
func (_m *API) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	ret := _m.Called(ctx, id)

	var r0 *flow.ExecutionResult
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.ExecutionResult); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ExecutionResult)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestBlock provides a mock function with given fields: ctx, isSealed
func (_m *API) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, error) {
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
func (_m *API) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, error) {
	ret := _m.Called(ctx, isSealed)

	var r0 *flow.Header
	if rf, ok := ret.Get(0).(func(context.Context, bool) *flow.Header); ok {
		r0 = rf(ctx, isSealed)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Header)
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

// NewAPI creates a new instance of API. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewAPI(t testing.TB) *API {
	mock := &API{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

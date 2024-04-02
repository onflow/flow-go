// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	context "context"

	flow "github.com/onflow/flow-go/model/flow"
	execution_data "github.com/onflow/flow-go/module/executiondatasync/execution_data"

	mock "github.com/stretchr/testify/mock"

	state_stream "github.com/onflow/flow-go/engine/access/state_stream"

	subscription "github.com/onflow/flow-go/engine/access/subscription"
)

// API is an autogenerated mock type for the API type
type API struct {
	mock.Mock
}

// GetExecutionDataByBlockID provides a mock function with given fields: ctx, blockID
func (_m *API) GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error) {
	ret := _m.Called(ctx, blockID)

	var r0 *execution_data.BlockExecutionData
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (*execution_data.BlockExecutionData, error)); ok {
		return rf(ctx, blockID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *execution_data.BlockExecutionData); ok {
		r0 = rf(ctx, blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*execution_data.BlockExecutionData)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRegisterValues provides a mock function with given fields: registerIDs, height
func (_m *API) GetRegisterValues(registerIDs flow.RegisterIDs, height uint64) ([][]byte, error) {
	ret := _m.Called(registerIDs, height)

	var r0 [][]byte
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.RegisterIDs, uint64) ([][]byte, error)); ok {
		return rf(registerIDs, height)
	}
	if rf, ok := ret.Get(0).(func(flow.RegisterIDs, uint64) [][]byte); ok {
		r0 = rf(registerIDs, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([][]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.RegisterIDs, uint64) error); ok {
		r1 = rf(registerIDs, height)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SubscribeAccountStatusesFromLatestBlock provides a mock function with given fields: ctx, filter
func (_m *API) SubscribeAccountStatusesFromLatestBlock(ctx context.Context, filter state_stream.AccountStatusFilter) subscription.Subscription {
	ret := _m.Called(ctx, filter)

	var r0 subscription.Subscription
	if rf, ok := ret.Get(0).(func(context.Context, state_stream.AccountStatusFilter) subscription.Subscription); ok {
		r0 = rf(ctx, filter)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(subscription.Subscription)
		}
	}

	return r0
}

// SubscribeAccountStatusesFromStartBlockID provides a mock function with given fields: ctx, startBlockID, filter
func (_m *API) SubscribeAccountStatusesFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, filter state_stream.AccountStatusFilter) subscription.Subscription {
	ret := _m.Called(ctx, startBlockID, filter)

	var r0 subscription.Subscription
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, state_stream.AccountStatusFilter) subscription.Subscription); ok {
		r0 = rf(ctx, startBlockID, filter)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(subscription.Subscription)
		}
	}

	return r0
}

// SubscribeAccountStatusesFromStartHeight provides a mock function with given fields: ctx, startHeight, filter
func (_m *API) SubscribeAccountStatusesFromStartHeight(ctx context.Context, startHeight uint64, filter state_stream.AccountStatusFilter) subscription.Subscription {
	ret := _m.Called(ctx, startHeight, filter)

	var r0 subscription.Subscription
	if rf, ok := ret.Get(0).(func(context.Context, uint64, state_stream.AccountStatusFilter) subscription.Subscription); ok {
		r0 = rf(ctx, startHeight, filter)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(subscription.Subscription)
		}
	}

	return r0
}

// SubscribeEvents provides a mock function with given fields: ctx, startBlockID, startHeight, filter
func (_m *API) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
	ret := _m.Called(ctx, startBlockID, startHeight, filter)

	var r0 subscription.Subscription
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, uint64, state_stream.EventFilter) subscription.Subscription); ok {
		r0 = rf(ctx, startBlockID, startHeight, filter)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(subscription.Subscription)
		}
	}

	return r0
}

// SubscribeExecutionData provides a mock function with given fields: ctx, startBlockID, startBlockHeight
func (_m *API) SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startBlockHeight uint64) subscription.Subscription {
	ret := _m.Called(ctx, startBlockID, startBlockHeight)

	var r0 subscription.Subscription
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, uint64) subscription.Subscription); ok {
		r0 = rf(ctx, startBlockID, startBlockHeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(subscription.Subscription)
		}
	}

	return r0
}

type mockConstructorTestingTNewAPI interface {
	mock.TestingT
	Cleanup(func())
}

// NewAPI creates a new instance of API. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewAPI(t mockConstructorTestingTNewAPI) *API {
	mock := &API{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

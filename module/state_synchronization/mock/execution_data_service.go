// Code generated by mockery v1.0.0. DO NOT EDIT.

package state_synchronization

import (
	context "context"

	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	state_synchronization "github.com/onflow/flow-go/module/state_synchronization"
)

// ExecutionDataService is an autogenerated mock type for the ExecutionDataService type
type ExecutionDataService struct {
	mock.Mock
}

// Add provides a mock function with given fields: ctx, sd
func (_m *ExecutionDataService) Add(ctx context.Context, sd *state_synchronization.ExecutionData) (flow.Identifier, state_synchronization.BlobTree, error) {
	ret := _m.Called(ctx, sd)

	var r0 flow.Identifier
	if rf, ok := ret.Get(0).(func(context.Context, *state_synchronization.ExecutionData) flow.Identifier); ok {
		r0 = rf(ctx, sd)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	var r1 state_synchronization.BlobTree
	if rf, ok := ret.Get(1).(func(context.Context, *state_synchronization.ExecutionData) state_synchronization.BlobTree); ok {
		r1 = rf(ctx, sd)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(state_synchronization.BlobTree)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context, *state_synchronization.ExecutionData) error); ok {
		r2 = rf(ctx, sd)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Get provides a mock function with given fields: ctx, rootID
func (_m *ExecutionDataService) Get(ctx context.Context, rootID flow.Identifier) (*state_synchronization.ExecutionData, error) {
	ret := _m.Called(ctx, rootID)

	var r0 *state_synchronization.ExecutionData
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *state_synchronization.ExecutionData); ok {
		r0 = rf(ctx, rootID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*state_synchronization.ExecutionData)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, rootID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

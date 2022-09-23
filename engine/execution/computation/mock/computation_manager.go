// Code generated by mockery v2.13.1. DO NOT EDIT.

package mock

import (
	context "context"

	execution "github.com/onflow/flow-go/engine/execution"
	entity "github.com/onflow/flow-go/module/mempool/entity"

	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"

	state "github.com/onflow/flow-go/fvm/state"
)

// ComputationManager is an autogenerated mock type for the ComputationManager type
type ComputationManager struct {
	mock.Mock
}

// ComputeBlock provides a mock function with given fields: ctx, block, view
func (_m *ComputationManager) ComputeBlock(ctx context.Context, block *entity.ExecutableBlock, view state.View) (*execution.ComputationResult, error) {
	ret := _m.Called(ctx, block, view)

	var r0 *execution.ComputationResult
	if rf, ok := ret.Get(0).(func(context.Context, *entity.ExecutableBlock, state.View) *execution.ComputationResult); ok {
		r0 = rf(ctx, block, view)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*execution.ComputationResult)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *entity.ExecutableBlock, state.View) error); ok {
		r1 = rf(ctx, block, view)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteScript provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4
func (_m *ComputationManager) ExecuteScript(_a0 context.Context, _a1 []byte, _a2 [][]byte, _a3 *flow.Header, _a4 state.View) ([]byte, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(context.Context, []byte, [][]byte, *flow.Header, state.View) []byte); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []byte, [][]byte, *flow.Header, state.View) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccount provides a mock function with given fields: addr, header, view
func (_m *ComputationManager) GetAccount(addr flow.Address, header *flow.Header, view state.View) (*flow.Account, error) {
	ret := _m.Called(addr, header, view)

	var r0 *flow.Account
	if rf, ok := ret.Get(0).(func(flow.Address, *flow.Header, state.View) *flow.Account); ok {
		r0 = rf(addr, header, view)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Account)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Address, *flow.Header, state.View) error); ok {
		r1 = rf(addr, header, view)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewComputationManager interface {
	mock.TestingT
	Cleanup(func())
}

// NewComputationManager creates a new instance of ComputationManager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewComputationManager(t mockConstructorTestingTNewComputationManager) *ComputationManager {
	mock := &ComputationManager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

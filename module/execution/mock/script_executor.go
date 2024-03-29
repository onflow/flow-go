// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	context "context"

	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// ScriptExecutor is an autogenerated mock type for the ScriptExecutor type
type ScriptExecutor struct {
	mock.Mock
}

// ExecuteAtBlockHeight provides a mock function with given fields: ctx, script, arguments, height
func (_m *ScriptExecutor) ExecuteAtBlockHeight(ctx context.Context, script []byte, arguments [][]byte, height uint64) ([]byte, error) {
	ret := _m.Called(ctx, script, arguments, height)

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []byte, [][]byte, uint64) ([]byte, error)); ok {
		return rf(ctx, script, arguments, height)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []byte, [][]byte, uint64) []byte); ok {
		r0 = rf(ctx, script, arguments, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []byte, [][]byte, uint64) error); ok {
		r1 = rf(ctx, script, arguments, height)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccountAtBlockHeight provides a mock function with given fields: ctx, address, height
func (_m *ScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	ret := _m.Called(ctx, address, height)

	var r0 *flow.Account
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address, uint64) (*flow.Account, error)); ok {
		return rf(ctx, address, height)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address, uint64) *flow.Account); ok {
		r0 = rf(ctx, address, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Account)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Address, uint64) error); ok {
		r1 = rf(ctx, address, height)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewScriptExecutor interface {
	mock.TestingT
	Cleanup(func())
}

// NewScriptExecutor creates a new instance of ScriptExecutor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewScriptExecutor(t mockConstructorTestingTNewScriptExecutor) *ScriptExecutor {
	mock := &ScriptExecutor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

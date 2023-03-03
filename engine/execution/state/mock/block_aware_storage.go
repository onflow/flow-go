// Code generated by mockery v2.13.1. DO NOT EDIT.

package mock

import (
	delta "github.com/onflow/flow-go/engine/execution/state/delta"
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// BlockAwareStorage is an autogenerated mock type for the BlockAwareStorage type
type BlockAwareStorage struct {
	mock.Mock
}

// OnBlockExecuted provides a mock function with given fields: block, _a1
func (_m *BlockAwareStorage) OnBlockExecuted(block *flow.Header, _a1 delta.SpockSnapshot) error {
	ret := _m.Called(block, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*flow.Header, delta.SpockSnapshot) error); ok {
		r0 = rf(block, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// OnFinalizedBlock provides a mock function with given fields: block
func (_m *BlockAwareStorage) OnFinalizedBlock(block *flow.Header) {
	_m.Called(block)
}

// OnSealedBlock provides a mock function with given fields: block
func (_m *BlockAwareStorage) OnSealedBlock(block *flow.Header) {
	_m.Called(block)
}

type mockConstructorTestingTNewBlockAwareStorage interface {
	mock.TestingT
	Cleanup(func())
}

// NewBlockAwareStorage creates a new instance of BlockAwareStorage. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewBlockAwareStorage(t mockConstructorTestingTNewBlockAwareStorage) *BlockAwareStorage {
	mock := &BlockAwareStorage{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

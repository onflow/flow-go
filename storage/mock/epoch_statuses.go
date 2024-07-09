// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	storage "github.com/onflow/flow-go/storage"

	transaction "github.com/onflow/flow-go/storage/badger/transaction"
)

// EpochStatuses is an autogenerated mock type for the EpochStatuses type
type EpochStatuses struct {
	mock.Mock
}

// ByBlockID provides a mock function with given fields: _a0
func (_m *EpochStatuses) ByBlockID(_a0 flow.Identifier) (*flow.EpochStatus, error) {
	ret := _m.Called(_a0)

	var r0 *flow.EpochStatus
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*flow.EpochStatus, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.EpochStatus); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.EpochStatus)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StorePebble provides a mock function with given fields: blockID, state
func (_m *EpochStatuses) StorePebble(blockID flow.Identifier, state *flow.EpochStatus) func(storage.PebbleReaderBatchWriter) error {
	ret := _m.Called(blockID, state)

	var r0 func(storage.PebbleReaderBatchWriter) error
	if rf, ok := ret.Get(0).(func(flow.Identifier, *flow.EpochStatus) func(storage.PebbleReaderBatchWriter) error); ok {
		r0 = rf(blockID, state)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(storage.PebbleReaderBatchWriter) error)
		}
	}

	return r0
}

// StoreTx provides a mock function with given fields: blockID, state
func (_m *EpochStatuses) StoreTx(blockID flow.Identifier, state *flow.EpochStatus) func(*transaction.Tx) error {
	ret := _m.Called(blockID, state)

	var r0 func(*transaction.Tx) error
	if rf, ok := ret.Get(0).(func(flow.Identifier, *flow.EpochStatus) func(*transaction.Tx) error); ok {
		r0 = rf(blockID, state)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*transaction.Tx) error)
		}
	}

	return r0
}

type mockConstructorTestingTNewEpochStatuses interface {
	mock.TestingT
	Cleanup(func())
}

// NewEpochStatuses creates a new instance of EpochStatuses. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewEpochStatuses(t mockConstructorTestingTNewEpochStatuses) *EpochStatuses {
	mock := &EpochStatuses{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

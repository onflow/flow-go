// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	transaction "github.com/onflow/flow-go/storage/badger/transaction"
)

// EpochProtocolStateEntries is an autogenerated mock type for the EpochProtocolStateEntries type
type EpochProtocolStateEntries struct {
	mock.Mock
}

// ByBlockID provides a mock function with given fields: blockID
func (_m *EpochProtocolStateEntries) ByBlockID(blockID flow.Identifier) (*flow.RichEpochProtocolStateEntry, error) {
	ret := _m.Called(blockID)

	if len(ret) == 0 {
		panic("no return value specified for ByBlockID")
	}

	var r0 *flow.RichEpochProtocolStateEntry
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*flow.RichEpochProtocolStateEntry, error)); ok {
		return rf(blockID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.RichEpochProtocolStateEntry); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.RichEpochProtocolStateEntry)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ByID provides a mock function with given fields: id
func (_m *EpochProtocolStateEntries) ByID(id flow.Identifier) (*flow.RichEpochProtocolStateEntry, error) {
	ret := _m.Called(id)

	if len(ret) == 0 {
		panic("no return value specified for ByID")
	}

	var r0 *flow.RichEpochProtocolStateEntry
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*flow.RichEpochProtocolStateEntry, error)); ok {
		return rf(id)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.RichEpochProtocolStateEntry); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.RichEpochProtocolStateEntry)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Index provides a mock function with given fields: blockID, epochProtocolStateID
func (_m *EpochProtocolStateEntries) Index(blockID flow.Identifier, epochProtocolStateID flow.Identifier) func(*transaction.Tx) error {
	ret := _m.Called(blockID, epochProtocolStateID)

	if len(ret) == 0 {
		panic("no return value specified for Index")
	}

	var r0 func(*transaction.Tx) error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) func(*transaction.Tx) error); ok {
		r0 = rf(blockID, epochProtocolStateID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*transaction.Tx) error)
		}
	}

	return r0
}

// StoreTx provides a mock function with given fields: epochProtocolStateID, epochProtocolStateEntry
func (_m *EpochProtocolStateEntries) StoreTx(epochProtocolStateID flow.Identifier, epochProtocolStateEntry *flow.EpochProtocolStateEntry) func(*transaction.Tx) error {
	ret := _m.Called(epochProtocolStateID, epochProtocolStateEntry)

	if len(ret) == 0 {
		panic("no return value specified for StoreTx")
	}

	var r0 func(*transaction.Tx) error
	if rf, ok := ret.Get(0).(func(flow.Identifier, *flow.EpochProtocolStateEntry) func(*transaction.Tx) error); ok {
		r0 = rf(epochProtocolStateID, epochProtocolStateEntry)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*transaction.Tx) error)
		}
	}

	return r0
}

// NewEpochProtocolStateEntries creates a new instance of EpochProtocolStateEntries. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewEpochProtocolStateEntries(t interface {
	mock.TestingT
	Cleanup(func())
}) *EpochProtocolStateEntries {
	mock := &EpochProtocolStateEntries{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

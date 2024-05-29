// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	protocol "github.com/onflow/flow-go/state/protocol"

	protocol_state "github.com/onflow/flow-go/state/protocol/protocol_state"

	transaction "github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolKVStore is an autogenerated mock type for the ProtocolKVStore type
type ProtocolKVStore struct {
	mock.Mock
}

// ByBlockID provides a mock function with given fields: blockID
func (_m *ProtocolKVStore) ByBlockID(blockID flow.Identifier) (protocol_state.KVStoreAPI, error) {
	ret := _m.Called(blockID)

	var r0 protocol_state.KVStoreAPI
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (protocol_state.KVStoreAPI, error)); ok {
		return rf(blockID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) protocol_state.KVStoreAPI); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol_state.KVStoreAPI)
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
func (_m *ProtocolKVStore) ByID(id flow.Identifier) (protocol_state.KVStoreAPI, error) {
	ret := _m.Called(id)

	var r0 protocol_state.KVStoreAPI
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (protocol_state.KVStoreAPI, error)); ok {
		return rf(id)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) protocol_state.KVStoreAPI); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol_state.KVStoreAPI)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// IndexTx provides a mock function with given fields: blockID, stateID
func (_m *ProtocolKVStore) IndexTx(blockID flow.Identifier, stateID flow.Identifier) func(*transaction.Tx) error {
	ret := _m.Called(blockID, stateID)

	var r0 func(*transaction.Tx) error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) func(*transaction.Tx) error); ok {
		r0 = rf(blockID, stateID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*transaction.Tx) error)
		}
	}

	return r0
}

// StoreTx provides a mock function with given fields: stateID, kvStore
func (_m *ProtocolKVStore) StoreTx(stateID flow.Identifier, kvStore protocol.KVStoreReader) func(*transaction.Tx) error {
	ret := _m.Called(stateID, kvStore)

	var r0 func(*transaction.Tx) error
	if rf, ok := ret.Get(0).(func(flow.Identifier, protocol.KVStoreReader) func(*transaction.Tx) error); ok {
		r0 = rf(stateID, kvStore)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*transaction.Tx) error)
		}
	}

	return r0
}

type mockConstructorTestingTNewProtocolKVStore interface {
	mock.TestingT
	Cleanup(func())
}

// NewProtocolKVStore creates a new instance of ProtocolKVStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewProtocolKVStore(t mockConstructorTestingTNewProtocolKVStore) *ProtocolKVStore {
	mock := &ProtocolKVStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

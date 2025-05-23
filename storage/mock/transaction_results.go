// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	storage "github.com/onflow/flow-go/storage"
)

// TransactionResults is an autogenerated mock type for the TransactionResults type
type TransactionResults struct {
	mock.Mock
}

// BatchRemoveByBlockID provides a mock function with given fields: id, batch
func (_m *TransactionResults) BatchRemoveByBlockID(id flow.Identifier, batch storage.ReaderBatchWriter) error {
	ret := _m.Called(id, batch)

	if len(ret) == 0 {
		panic("no return value specified for BatchRemoveByBlockID")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, storage.ReaderBatchWriter) error); ok {
		r0 = rf(id, batch)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// BatchStore provides a mock function with given fields: blockID, transactionResults, batch
func (_m *TransactionResults) BatchStore(blockID flow.Identifier, transactionResults []flow.TransactionResult, batch storage.ReaderBatchWriter) error {
	ret := _m.Called(blockID, transactionResults, batch)

	if len(ret) == 0 {
		panic("no return value specified for BatchStore")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, []flow.TransactionResult, storage.ReaderBatchWriter) error); ok {
		r0 = rf(blockID, transactionResults, batch)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ByBlockID provides a mock function with given fields: id
func (_m *TransactionResults) ByBlockID(id flow.Identifier) ([]flow.TransactionResult, error) {
	ret := _m.Called(id)

	if len(ret) == 0 {
		panic("no return value specified for ByBlockID")
	}

	var r0 []flow.TransactionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) ([]flow.TransactionResult, error)); ok {
		return rf(id)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) []flow.TransactionResult); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.TransactionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ByBlockIDTransactionID provides a mock function with given fields: blockID, transactionID
func (_m *TransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResult, error) {
	ret := _m.Called(blockID, transactionID)

	if len(ret) == 0 {
		panic("no return value specified for ByBlockIDTransactionID")
	}

	var r0 *flow.TransactionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) (*flow.TransactionResult, error)); ok {
		return rf(blockID, transactionID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) *flow.TransactionResult); ok {
		r0 = rf(blockID, transactionID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.TransactionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier, flow.Identifier) error); ok {
		r1 = rf(blockID, transactionID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ByBlockIDTransactionIndex provides a mock function with given fields: blockID, txIndex
func (_m *TransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResult, error) {
	ret := _m.Called(blockID, txIndex)

	if len(ret) == 0 {
		panic("no return value specified for ByBlockIDTransactionIndex")
	}

	var r0 *flow.TransactionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, uint32) (*flow.TransactionResult, error)); ok {
		return rf(blockID, txIndex)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier, uint32) *flow.TransactionResult); ok {
		r0 = rf(blockID, txIndex)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.TransactionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier, uint32) error); ok {
		r1 = rf(blockID, txIndex)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewTransactionResults creates a new instance of TransactionResults. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewTransactionResults(t interface {
	mock.TestingT
	Cleanup(func())
}) *TransactionResults {
	mock := &TransactionResults{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

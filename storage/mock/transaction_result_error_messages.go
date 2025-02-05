// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// TransactionResultErrorMessages is an autogenerated mock type for the TransactionResultErrorMessages type
type TransactionResultErrorMessages struct {
	mock.Mock
}

// ByBlockID provides a mock function with given fields: id
func (_m *TransactionResultErrorMessages) ByBlockID(id flow.Identifier) ([]flow.TransactionResultErrorMessage, error) {
	ret := _m.Called(id)

	var r0 []flow.TransactionResultErrorMessage
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) ([]flow.TransactionResultErrorMessage, error)); ok {
		return rf(id)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) []flow.TransactionResultErrorMessage); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.TransactionResultErrorMessage)
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
func (_m *TransactionResultErrorMessages) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResultErrorMessage, error) {
	ret := _m.Called(blockID, transactionID)

	var r0 *flow.TransactionResultErrorMessage
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) (*flow.TransactionResultErrorMessage, error)); ok {
		return rf(blockID, transactionID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) *flow.TransactionResultErrorMessage); ok {
		r0 = rf(blockID, transactionID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.TransactionResultErrorMessage)
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
func (_m *TransactionResultErrorMessages) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResultErrorMessage, error) {
	ret := _m.Called(blockID, txIndex)

	var r0 *flow.TransactionResultErrorMessage
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, uint32) (*flow.TransactionResultErrorMessage, error)); ok {
		return rf(blockID, txIndex)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier, uint32) *flow.TransactionResultErrorMessage); ok {
		r0 = rf(blockID, txIndex)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.TransactionResultErrorMessage)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier, uint32) error); ok {
		r1 = rf(blockID, txIndex)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Exists provides a mock function with given fields: blockID
func (_m *TransactionResultErrorMessages) Exists(blockID flow.Identifier) (bool, error) {
	ret := _m.Called(blockID)

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (bool, error)); ok {
		return rf(blockID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(blockID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Store provides a mock function with given fields: blockID, transactionResultErrorMessages
func (_m *TransactionResultErrorMessages) Store(blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage) error {
	ret := _m.Called(blockID, transactionResultErrorMessages)

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, []flow.TransactionResultErrorMessage) error); ok {
		r0 = rf(blockID, transactionResultErrorMessages)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewTransactionResultErrorMessages interface {
	mock.TestingT
	Cleanup(func())
}

// NewTransactionResultErrorMessages creates a new instance of TransactionResultErrorMessages. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewTransactionResultErrorMessages(t mockConstructorTestingTNewTransactionResultErrorMessages) *TransactionResultErrorMessages {
	mock := &TransactionResultErrorMessages{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

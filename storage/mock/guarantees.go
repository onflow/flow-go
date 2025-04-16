// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	transaction "github.com/onflow/flow-go/storage/badger/transaction"
)

// Guarantees is an autogenerated mock type for the Guarantees type
type Guarantees struct {
	mock.Mock
}

// ByCollectionID provides a mock function with given fields: collID
func (_m *Guarantees) ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	ret := _m.Called(collID)

	if len(ret) == 0 {
		panic("no return value specified for ByCollectionID")
	}

	var r0 *flow.CollectionGuarantee
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*flow.CollectionGuarantee, error)); ok {
		return rf(collID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.CollectionGuarantee); ok {
		r0 = rf(collID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.CollectionGuarantee)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(collID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ByID provides a mock function with given fields: guaranteeID
func (_m *Guarantees) ByID(guaranteeID flow.Identifier) (*flow.CollectionGuarantee, error) {
	ret := _m.Called(guaranteeID)

	if len(ret) == 0 {
		panic("no return value specified for ByID")
	}

	var r0 *flow.CollectionGuarantee
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*flow.CollectionGuarantee, error)); ok {
		return rf(guaranteeID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.CollectionGuarantee); ok {
		r0 = rf(guaranteeID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.CollectionGuarantee)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(guaranteeID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Index provides a mock function with given fields: collID, guaranteeID
func (_m *Guarantees) Index(collID flow.Identifier, guaranteeID flow.Identifier) func(*transaction.Tx) error {
	ret := _m.Called(collID, guaranteeID)

	if len(ret) == 0 {
		panic("no return value specified for Index")
	}

	var r0 func(*transaction.Tx) error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) func(*transaction.Tx) error); ok {
		r0 = rf(collID, guaranteeID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*transaction.Tx) error)
		}
	}

	return r0
}

// Store provides a mock function with given fields: guarantee
func (_m *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	ret := _m.Called(guarantee)

	if len(ret) == 0 {
		panic("no return value specified for Store")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*flow.CollectionGuarantee) error); ok {
		r0 = rf(guarantee)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewGuarantees creates a new instance of Guarantees. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewGuarantees(t interface {
	mock.TestingT
	Cleanup(func())
}) *Guarantees {
	mock := &Guarantees{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	ledger "github.com/onflow/flow-go/ledger"
	mock "github.com/stretchr/testify/mock"
)

// Migration is an autogenerated mock type for the Migration type
type Migration struct {
	mock.Mock
}

// Execute provides a mock function with given fields: payloads
func (_m *Migration) Execute(payloads []*ledger.Payload) ([]*ledger.Payload, error) {
	ret := _m.Called(payloads)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 []*ledger.Payload
	var r1 error
	if rf, ok := ret.Get(0).(func([]*ledger.Payload) ([]*ledger.Payload, error)); ok {
		return rf(payloads)
	}
	if rf, ok := ret.Get(0).(func([]*ledger.Payload) []*ledger.Payload); ok {
		r0 = rf(payloads)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*ledger.Payload)
		}
	}

	if rf, ok := ret.Get(1).(func([]*ledger.Payload) error); ok {
		r1 = rf(payloads)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMigration creates a new instance of Migration. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMigration(t interface {
	mock.TestingT
	Cleanup(func())
}) *Migration {
	mock := &Migration{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

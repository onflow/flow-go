// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// Transaction is an autogenerated mock type for the Transaction type
type Transaction struct {
	mock.Mock
}

// Set provides a mock function with given fields: key, val
func (_m *Transaction) Set(key []byte, val []byte) error {
	ret := _m.Called(key, val)

	if len(ret) == 0 {
		panic("no return value specified for Set")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func([]byte, []byte) error); ok {
		r0 = rf(key, val)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewTransaction creates a new instance of Transaction. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewTransaction(t interface {
	mock.TestingT
	Cleanup(func())
}) *Transaction {
	mock := &Transaction{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

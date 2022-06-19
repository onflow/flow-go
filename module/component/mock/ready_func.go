// Code generated by mockery v2.13.0. DO NOT EDIT.

package component

import mock "github.com/stretchr/testify/mock"

// ReadyFunc is an autogenerated mock type for the ReadyFunc type
type ReadyFunc struct {
	mock.Mock
}

// Execute provides a mock function with given fields:
func (_m *ReadyFunc) Execute() {
	_m.Called()
}

type NewReadyFuncT interface {
	mock.TestingT
	Cleanup(func())
}

// NewReadyFunc creates a new instance of ReadyFunc. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewReadyFunc(t NewReadyFuncT) *ReadyFunc {
	mock := &ReadyFunc{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

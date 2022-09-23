// Code generated by mockery v2.13.1. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// Workerpool is an autogenerated mock type for the Workerpool type
type Workerpool struct {
	mock.Mock
}

// StopWait provides a mock function with given fields:
func (_m *Workerpool) StopWait() {
	_m.Called()
}

// Submit provides a mock function with given fields: task
func (_m *Workerpool) Submit(task func()) {
	_m.Called(task)
}

type mockConstructorTestingTNewWorkerpool interface {
	mock.TestingT
	Cleanup(func())
}

// NewWorkerpool creates a new instance of Workerpool. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewWorkerpool(t mockConstructorTestingTNewWorkerpool) *Workerpool {
	mock := &Workerpool{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

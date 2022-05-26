// Code generated by mockery v2.12.3. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// Workers is an autogenerated mock type for the Workers type
type Workers struct {
	mock.Mock
}

// Submit provides a mock function with given fields: task
func (_m *Workers) Submit(task func()) {
	_m.Called(task)
}

type NewWorkersT interface {
	mock.TestingT
	Cleanup(func())
}

// NewWorkers creates a new instance of Workers. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewWorkers(t NewWorkersT) *Workers {
	mock := &Workers{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

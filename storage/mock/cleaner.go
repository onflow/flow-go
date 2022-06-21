// Code generated by mockery v2.13.0. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// Cleaner is an autogenerated mock type for the Cleaner type
type Cleaner struct {
	mock.Mock
}

// RunGC provides a mock function with given fields:
func (_m *Cleaner) RunGC() {
	_m.Called()
}

type NewCleanerT interface {
	mock.TestingT
	Cleanup(func())
}

// NewCleaner creates a new instance of Cleaner. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewCleaner(t NewCleanerT) *Cleaner {
	mock := &Cleaner{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

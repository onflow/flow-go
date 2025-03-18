// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// OnEntityEjected is an autogenerated mock type for the OnEntityEjected type
type OnEntityEjected[V interface{}] struct {
	mock.Mock
}

// Execute provides a mock function with given fields: ejectedEntity
func (_m *OnEntityEjected[V]) Execute(ejectedEntity V) {
	_m.Called(ejectedEntity)
}

// NewOnEntityEjected creates a new instance of OnEntityEjected. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewOnEntityEjected[V interface{}](t interface {
	mock.TestingT
	Cleanup(func())
}) *OnEntityEjected[V] {
	mock := &OnEntityEjected[V]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// RegisterStoreNotifier is an autogenerated mock type for the RegisterStoreNotifier type
type RegisterStoreNotifier struct {
	mock.Mock
}

// OnFinalizedAndExecutedHeightUpdated provides a mock function with given fields: height
func (_m *RegisterStoreNotifier) OnFinalizedAndExecutedHeightUpdated(height uint64) {
	_m.Called(height)
}

type mockConstructorTestingTNewRegisterStoreNotifier interface {
	mock.TestingT
	Cleanup(func())
}

// NewRegisterStoreNotifier creates a new instance of RegisterStoreNotifier. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewRegisterStoreNotifier(t mockConstructorTestingTNewRegisterStoreNotifier) *RegisterStoreNotifier {
	mock := &RegisterStoreNotifier{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

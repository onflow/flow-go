// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// WALMetrics is an autogenerated mock type for the WALMetrics type
type WALMetrics struct {
	mock.Mock
}

type mockConstructorTestingTNewWALMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewWALMetrics creates a new instance of WALMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewWALMetrics(t mockConstructorTestingTNewWALMetrics) *WALMetrics {
	mock := &WALMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

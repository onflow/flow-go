// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// CruiseCtlMetrics is an autogenerated mock type for the CruiseCtlMetrics type
type CruiseCtlMetrics struct {
	mock.Mock
}

// ControllerOutput provides a mock function with given fields: duration
func (_m *CruiseCtlMetrics) ControllerOutput(duration time.Duration) {
	_m.Called(duration)
}

// PIDError provides a mock function with given fields: p, i, d
func (_m *CruiseCtlMetrics) PIDError(p float64, i float64, d float64) {
	_m.Called(p, i, d)
}

// TargetProposalDuration provides a mock function with given fields: duration
func (_m *CruiseCtlMetrics) TargetProposalDuration(duration time.Duration) {
	_m.Called(duration)
}

type mockConstructorTestingTNewCruiseCtlMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewCruiseCtlMetrics creates a new instance of CruiseCtlMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewCruiseCtlMetrics(t mockConstructorTestingTNewCruiseCtlMetrics) *CruiseCtlMetrics {
	mock := &CruiseCtlMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

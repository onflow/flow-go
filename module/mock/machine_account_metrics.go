// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// MachineAccountMetrics is an autogenerated mock type for the MachineAccountMetrics type
type MachineAccountMetrics struct {
	mock.Mock
}

// AccountBalance provides a mock function with given fields: bal
func (_m *MachineAccountMetrics) AccountBalance(bal float64) {
	_m.Called(bal)
}

// IsMisconfigured provides a mock function with given fields: misconfigured
func (_m *MachineAccountMetrics) IsMisconfigured(misconfigured bool) {
	_m.Called(misconfigured)
}

// RecommendedMinBalance provides a mock function with given fields: bal
func (_m *MachineAccountMetrics) RecommendedMinBalance(bal float64) {
	_m.Called(bal)
}

type mockConstructorTestingTNewMachineAccountMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewMachineAccountMetrics creates a new instance of MachineAccountMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMachineAccountMetrics(t mockConstructorTestingTNewMachineAccountMetrics) *MachineAccountMetrics {
	mock := &MachineAccountMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
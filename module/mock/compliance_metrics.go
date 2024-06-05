// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// ComplianceMetrics is an autogenerated mock type for the ComplianceMetrics type
type ComplianceMetrics struct {
	mock.Mock
}

// BlockFinalized provides a mock function with given fields: _a0
func (_m *ComplianceMetrics) BlockFinalized(_a0 *flow.Block) {
	_m.Called(_a0)
}

// BlockSealed provides a mock function with given fields: _a0
func (_m *ComplianceMetrics) BlockSealed(_a0 *flow.Block) {
	_m.Called(_a0)
}

// CurrentDKGPhase1FinalView provides a mock function with given fields: view
func (_m *ComplianceMetrics) CurrentDKGPhase1FinalView(view uint64) {
	_m.Called(view)
}

// CurrentDKGPhase2FinalView provides a mock function with given fields: view
func (_m *ComplianceMetrics) CurrentDKGPhase2FinalView(view uint64) {
	_m.Called(view)
}

// CurrentDKGPhase3FinalView provides a mock function with given fields: view
func (_m *ComplianceMetrics) CurrentDKGPhase3FinalView(view uint64) {
	_m.Called(view)
}

// CurrentEpochCounter provides a mock function with given fields: counter
func (_m *ComplianceMetrics) CurrentEpochCounter(counter uint64) {
	_m.Called(counter)
}

// CurrentEpochFinalView provides a mock function with given fields: view
func (_m *ComplianceMetrics) CurrentEpochFinalView(view uint64) {
	_m.Called(view)
}

// CurrentEpochPhase provides a mock function with given fields: phase
func (_m *ComplianceMetrics) CurrentEpochPhase(phase flow.EpochPhase) {
	_m.Called(phase)
}

// EpochEmergencyFallbackTriggered provides a mock function with given fields:
func (_m *ComplianceMetrics) EpochEmergencyFallbackTriggered() {
	_m.Called()
}

// EpochTransitionHeight provides a mock function with given fields: height
func (_m *ComplianceMetrics) EpochTransitionHeight(height uint64) {
	_m.Called(height)
}

// FinalizedHeight provides a mock function with given fields: height
func (_m *ComplianceMetrics) FinalizedHeight(height uint64) {
	_m.Called(height)
}

// SealedHeight provides a mock function with given fields: height
func (_m *ComplianceMetrics) SealedHeight(height uint64) {
	_m.Called(height)
}

// NewComplianceMetrics creates a new instance of ComplianceMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewComplianceMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *ComplianceMetrics {
	mock := &ComplianceMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

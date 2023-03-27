// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// HotstuffMetrics is an autogenerated mock type for the HotstuffMetrics type
type HotstuffMetrics struct {
	mock.Mock
}

// BlockProcessingDuration provides a mock function with given fields: duration
func (_m *HotstuffMetrics) BlockProcessingDuration(duration time.Duration) {
	_m.Called(duration)
}

// CommitteeProcessingDuration provides a mock function with given fields: duration
func (_m *HotstuffMetrics) CommitteeProcessingDuration(duration time.Duration) {
	_m.Called(duration)
}

// CountSkipped provides a mock function with given fields:
func (_m *HotstuffMetrics) CountSkipped() {
	_m.Called()
}

// CountTimeout provides a mock function with given fields:
func (_m *HotstuffMetrics) CountTimeout() {
	_m.Called()
}

// HotStuffBusyDuration provides a mock function with given fields: duration, event
func (_m *HotstuffMetrics) HotStuffBusyDuration(duration time.Duration, event string) {
	_m.Called(duration, event)
}

// HotStuffIdleDuration provides a mock function with given fields: duration
func (_m *HotstuffMetrics) HotStuffIdleDuration(duration time.Duration) {
	_m.Called(duration)
}

// HotStuffWaitDuration provides a mock function with given fields: duration, event
func (_m *HotstuffMetrics) HotStuffWaitDuration(duration time.Duration, event string) {
	_m.Called(duration, event)
}

// PayloadProductionDuration provides a mock function with given fields: duration
func (_m *HotstuffMetrics) PayloadProductionDuration(duration time.Duration) {
	_m.Called(duration)
}

// SetCurView provides a mock function with given fields: view
func (_m *HotstuffMetrics) SetCurView(view uint64) {
	_m.Called(view)
}

// SetQCView provides a mock function with given fields: view
func (_m *HotstuffMetrics) SetQCView(view uint64) {
	_m.Called(view)
}

// SetTCView provides a mock function with given fields: view
func (_m *HotstuffMetrics) SetTCView(view uint64) {
	_m.Called(view)
}

// SetTimeout provides a mock function with given fields: duration
func (_m *HotstuffMetrics) SetTimeout(duration time.Duration) {
	_m.Called(duration)
}

// SignerProcessingDuration provides a mock function with given fields: duration
func (_m *HotstuffMetrics) SignerProcessingDuration(duration time.Duration) {
	_m.Called(duration)
}

// TimeoutObjectProcessingDuration provides a mock function with given fields: duration
func (_m *HotstuffMetrics) TimeoutObjectProcessingDuration(duration time.Duration) {
	_m.Called(duration)
}

// ValidatorProcessingDuration provides a mock function with given fields: duration
func (_m *HotstuffMetrics) ValidatorProcessingDuration(duration time.Duration) {
	_m.Called(duration)
}

// VoteProcessingDuration provides a mock function with given fields: duration
func (_m *HotstuffMetrics) VoteProcessingDuration(duration time.Duration) {
	_m.Called(duration)
}

type mockConstructorTestingTNewHotstuffMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewHotstuffMetrics creates a new instance of HotstuffMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewHotstuffMetrics(t mockConstructorTestingTNewHotstuffMetrics) *HotstuffMetrics {
	mock := &HotstuffMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

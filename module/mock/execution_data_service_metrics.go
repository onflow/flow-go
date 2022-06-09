// Code generated by mockery v2.12.3. DO NOT EDIT.

package mock

import (
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// ExecutionDataServiceMetrics is an autogenerated mock type for the ExecutionDataServiceMetrics type
type ExecutionDataServiceMetrics struct {
	mock.Mock
}

// ExecutionDataAddFinished provides a mock function with given fields: duration, success, blobTreeSize
func (_m *ExecutionDataServiceMetrics) ExecutionDataAddFinished(duration time.Duration, success bool, blobTreeSize uint64) {
	_m.Called(duration, success, blobTreeSize)
}

// ExecutionDataAddStarted provides a mock function with given fields:
func (_m *ExecutionDataServiceMetrics) ExecutionDataAddStarted() {
	_m.Called()
}

// ExecutionDataGetFinished provides a mock function with given fields: duration, success, blobTreeSize
func (_m *ExecutionDataServiceMetrics) ExecutionDataGetFinished(duration time.Duration, success bool, blobTreeSize uint64) {
	_m.Called(duration, success, blobTreeSize)
}

// ExecutionDataGetStarted provides a mock function with given fields:
func (_m *ExecutionDataServiceMetrics) ExecutionDataGetStarted() {
	_m.Called()
}

type NewExecutionDataServiceMetricsT interface {
	mock.TestingT
	Cleanup(func())
}

// NewExecutionDataServiceMetrics creates a new instance of ExecutionDataServiceMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewExecutionDataServiceMetrics(t NewExecutionDataServiceMetricsT) *ExecutionDataServiceMetrics {
	mock := &ExecutionDataServiceMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

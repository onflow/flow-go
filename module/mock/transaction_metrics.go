// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// TransactionMetrics is an autogenerated mock type for the TransactionMetrics type
type TransactionMetrics struct {
	mock.Mock
}

// ConnectionFromPoolRetrieved provides a mock function with given fields:
func (_m *TransactionMetrics) ConnectionFromPoolRetrieved() {
	_m.Called()
}

// TotalConnectionsInPool provides a mock function with given fields: connectionCount, connectionPoolSize
func (_m *TransactionMetrics) TotalConnectionsInPool(connectionCount uint, connectionPoolSize uint) {
	_m.Called(connectionCount, connectionPoolSize)
}

// TransactionExecuted provides a mock function with given fields: txID, when
func (_m *TransactionMetrics) TransactionExecuted(txID flow.Identifier, when time.Time) {
	_m.Called(txID, when)
}

// TransactionExpired provides a mock function with given fields: txID
func (_m *TransactionMetrics) TransactionExpired(txID flow.Identifier) {
	_m.Called(txID)
}

// TransactionFinalized provides a mock function with given fields: txID, when
func (_m *TransactionMetrics) TransactionFinalized(txID flow.Identifier, when time.Time) {
	_m.Called(txID, when)
}

// TransactionReceived provides a mock function with given fields: txID, when
func (_m *TransactionMetrics) TransactionReceived(txID flow.Identifier, when time.Time) {
	_m.Called(txID, when)
}

// TransactionSubmissionFailed provides a mock function with given fields:
func (_m *TransactionMetrics) TransactionSubmissionFailed() {
	_m.Called()
}

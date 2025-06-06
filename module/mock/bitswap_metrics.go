// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// BitswapMetrics is an autogenerated mock type for the BitswapMetrics type
type BitswapMetrics struct {
	mock.Mock
}

// BlobsReceived provides a mock function with given fields: prefix, n
func (_m *BitswapMetrics) BlobsReceived(prefix string, n uint64) {
	_m.Called(prefix, n)
}

// BlobsSent provides a mock function with given fields: prefix, n
func (_m *BitswapMetrics) BlobsSent(prefix string, n uint64) {
	_m.Called(prefix, n)
}

// DataReceived provides a mock function with given fields: prefix, n
func (_m *BitswapMetrics) DataReceived(prefix string, n uint64) {
	_m.Called(prefix, n)
}

// DataSent provides a mock function with given fields: prefix, n
func (_m *BitswapMetrics) DataSent(prefix string, n uint64) {
	_m.Called(prefix, n)
}

// DupBlobsReceived provides a mock function with given fields: prefix, n
func (_m *BitswapMetrics) DupBlobsReceived(prefix string, n uint64) {
	_m.Called(prefix, n)
}

// DupDataReceived provides a mock function with given fields: prefix, n
func (_m *BitswapMetrics) DupDataReceived(prefix string, n uint64) {
	_m.Called(prefix, n)
}

// MessagesReceived provides a mock function with given fields: prefix, n
func (_m *BitswapMetrics) MessagesReceived(prefix string, n uint64) {
	_m.Called(prefix, n)
}

// Peers provides a mock function with given fields: prefix, n
func (_m *BitswapMetrics) Peers(prefix string, n int) {
	_m.Called(prefix, n)
}

// Wantlist provides a mock function with given fields: prefix, n
func (_m *BitswapMetrics) Wantlist(prefix string, n int) {
	_m.Called(prefix, n)
}

// NewBitswapMetrics creates a new instance of BitswapMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewBitswapMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *BitswapMetrics {
	mock := &BitswapMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

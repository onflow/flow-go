// Code generated by mockery v2.13.1. DO NOT EDIT.

package mock

import (
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// NetworkMetrics is an autogenerated mock type for the NetworkMetrics type
type NetworkMetrics struct {
	mock.Mock
}

// DNSLookupDuration provides a mock function with given fields: duration
func (_m *NetworkMetrics) DNSLookupDuration(duration time.Duration) {
	_m.Called(duration)
}

// DirectMessageFinished provides a mock function with given fields: topic
func (_m *NetworkMetrics) DirectMessageFinished(topic string) {
	_m.Called(topic)
}

// DirectMessageStarted provides a mock function with given fields: topic
func (_m *NetworkMetrics) DirectMessageStarted(topic string) {
	_m.Called(topic)
}

// InboundConnections provides a mock function with given fields: connectionCount
func (_m *NetworkMetrics) InboundConnections(connectionCount uint) {
	_m.Called(connectionCount)
}

// MessageAdded provides a mock function with given fields: priority
func (_m *NetworkMetrics) MessageAdded(priority int) {
	_m.Called(priority)
}

// MessageProcessingFinished provides a mock function with given fields: topic, duration
func (_m *NetworkMetrics) MessageProcessingFinished(topic string, duration time.Duration) {
	_m.Called(topic, duration)
}

// MessageProcessingStarted provides a mock function with given fields: topic
func (_m *NetworkMetrics) MessageProcessingStarted(topic string) {
	_m.Called(topic)
}

// MessageRemoved provides a mock function with given fields: priority
func (_m *NetworkMetrics) MessageRemoved(priority int) {
	_m.Called(priority)
}

// NetworkDuplicateMessagesDropped provides a mock function with given fields: topic, messageType
func (_m *NetworkMetrics) NetworkDuplicateMessagesDropped(topic string, messageType string) {
	_m.Called(topic, messageType)
}

// NetworkMessageReceived provides a mock function with given fields: sizeBytes, topic, messageType
func (_m *NetworkMetrics) NetworkMessageReceived(sizeBytes int, topic string, messageType string) {
	_m.Called(sizeBytes, topic, messageType)
}

// NetworkMessageSent provides a mock function with given fields: sizeBytes, topic, messageType
func (_m *NetworkMetrics) NetworkMessageSent(sizeBytes int, topic string, messageType string) {
	_m.Called(sizeBytes, topic, messageType)
}

// OnDNSCacheHit provides a mock function with given fields:
func (_m *NetworkMetrics) OnDNSCacheHit() {
	_m.Called()
}

// OnDNSCacheInvalidated provides a mock function with given fields:
func (_m *NetworkMetrics) OnDNSCacheInvalidated() {
	_m.Called()
}

// OnDNSCacheMiss provides a mock function with given fields:
func (_m *NetworkMetrics) OnDNSCacheMiss() {
	_m.Called()
}

// OnDNSLookupRequestDropped provides a mock function with given fields:
func (_m *NetworkMetrics) OnDNSLookupRequestDropped() {
	_m.Called()
}

// OnGraftReceived provides a mock function with given fields: topic
func (_m *NetworkMetrics) OnGraftReceived(topic string) {
	_m.Called(topic)
}

// OnIHaveReceived provides a mock function with given fields: topic
func (_m *NetworkMetrics) OnIHaveReceived(topic string) {
	_m.Called(topic)
}

// OnIWantReceived provides a mock function with given fields: topic
func (_m *NetworkMetrics) OnIWantReceived(topic string) {
	_m.Called(topic)
}

// OnPruneReceived provides a mock function with given fields: topic
func (_m *NetworkMetrics) OnPruneReceived(topic string) {
	_m.Called(topic)
}

// OnRateLimitedUnicastMessage provides a mock function with given fields: role, msgType, topic, reason
func (_m *NetworkMetrics) OnRateLimitedUnicastMessage(role string, msgType string, topic string, reason string) {
	_m.Called(role, msgType, topic, reason)
}

// OnUnauthorizedMessage provides a mock function with given fields: role, msgType, topic, offense
func (_m *NetworkMetrics) OnUnauthorizedMessage(role string, msgType string, topic string, offense string) {
	_m.Called(role, msgType, topic, offense)
}

// OutboundConnections provides a mock function with given fields: connectionCount
func (_m *NetworkMetrics) OutboundConnections(connectionCount uint) {
	_m.Called(connectionCount)
}

// QueueDuration provides a mock function with given fields: duration, priority
func (_m *NetworkMetrics) QueueDuration(duration time.Duration, priority int) {
	_m.Called(duration, priority)
}

// RoutingTablePeerAdded provides a mock function with given fields:
func (_m *NetworkMetrics) RoutingTablePeerAdded() {
	_m.Called()
}

// RoutingTablePeerRemoved provides a mock function with given fields:
func (_m *NetworkMetrics) RoutingTablePeerRemoved() {
	_m.Called()
}

type mockConstructorTestingTNewNetworkMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewNetworkMetrics creates a new instance of NetworkMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewNetworkMetrics(t mockConstructorTestingTNewNetworkMetrics) *NetworkMetrics {
	mock := &NetworkMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

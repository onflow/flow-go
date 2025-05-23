// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// LocalGossipSubRouterMetrics is an autogenerated mock type for the LocalGossipSubRouterMetrics type
type LocalGossipSubRouterMetrics struct {
	mock.Mock
}

// OnLocalMeshSizeUpdated provides a mock function with given fields: topic, size
func (_m *LocalGossipSubRouterMetrics) OnLocalMeshSizeUpdated(topic string, size int) {
	_m.Called(topic, size)
}

// OnLocalPeerJoinedTopic provides a mock function with no fields
func (_m *LocalGossipSubRouterMetrics) OnLocalPeerJoinedTopic() {
	_m.Called()
}

// OnLocalPeerLeftTopic provides a mock function with no fields
func (_m *LocalGossipSubRouterMetrics) OnLocalPeerLeftTopic() {
	_m.Called()
}

// OnMessageDeliveredToAllSubscribers provides a mock function with given fields: size
func (_m *LocalGossipSubRouterMetrics) OnMessageDeliveredToAllSubscribers(size int) {
	_m.Called(size)
}

// OnMessageDuplicate provides a mock function with given fields: size
func (_m *LocalGossipSubRouterMetrics) OnMessageDuplicate(size int) {
	_m.Called(size)
}

// OnMessageEnteredValidation provides a mock function with given fields: size
func (_m *LocalGossipSubRouterMetrics) OnMessageEnteredValidation(size int) {
	_m.Called(size)
}

// OnMessageRejected provides a mock function with given fields: size, reason
func (_m *LocalGossipSubRouterMetrics) OnMessageRejected(size int, reason string) {
	_m.Called(size, reason)
}

// OnOutboundRpcDropped provides a mock function with no fields
func (_m *LocalGossipSubRouterMetrics) OnOutboundRpcDropped() {
	_m.Called()
}

// OnPeerAddedToProtocol provides a mock function with given fields: protocol
func (_m *LocalGossipSubRouterMetrics) OnPeerAddedToProtocol(protocol string) {
	_m.Called(protocol)
}

// OnPeerGraftTopic provides a mock function with given fields: topic
func (_m *LocalGossipSubRouterMetrics) OnPeerGraftTopic(topic string) {
	_m.Called(topic)
}

// OnPeerPruneTopic provides a mock function with given fields: topic
func (_m *LocalGossipSubRouterMetrics) OnPeerPruneTopic(topic string) {
	_m.Called(topic)
}

// OnPeerRemovedFromProtocol provides a mock function with no fields
func (_m *LocalGossipSubRouterMetrics) OnPeerRemovedFromProtocol() {
	_m.Called()
}

// OnPeerThrottled provides a mock function with no fields
func (_m *LocalGossipSubRouterMetrics) OnPeerThrottled() {
	_m.Called()
}

// OnRpcReceived provides a mock function with given fields: msgCount, iHaveCount, iWantCount, graftCount, pruneCount
func (_m *LocalGossipSubRouterMetrics) OnRpcReceived(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int) {
	_m.Called(msgCount, iHaveCount, iWantCount, graftCount, pruneCount)
}

// OnRpcSent provides a mock function with given fields: msgCount, iHaveCount, iWantCount, graftCount, pruneCount
func (_m *LocalGossipSubRouterMetrics) OnRpcSent(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int) {
	_m.Called(msgCount, iHaveCount, iWantCount, graftCount, pruneCount)
}

// OnUndeliveredMessage provides a mock function with no fields
func (_m *LocalGossipSubRouterMetrics) OnUndeliveredMessage() {
	_m.Called()
}

// NewLocalGossipSubRouterMetrics creates a new instance of LocalGossipSubRouterMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewLocalGossipSubRouterMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *LocalGossipSubRouterMetrics {
	mock := &LocalGossipSubRouterMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

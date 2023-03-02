// Code generated by mockery v2.13.1. DO NOT EDIT.

package mock

import (
	channels "github.com/onflow/flow-go/network/channels"
	mock "github.com/stretchr/testify/mock"

	network "github.com/libp2p/go-libp2p/core/network"

	peer "github.com/libp2p/go-libp2p/core/peer"

	protocol "github.com/libp2p/go-libp2p/core/protocol"

	time "time"
)

// LibP2PMetrics is an autogenerated mock type for the LibP2PMetrics type
type LibP2PMetrics struct {
	mock.Mock
}

// AllowConn provides a mock function with given fields: dir, usefd
func (_m *LibP2PMetrics) AllowConn(dir network.Direction, usefd bool) {
	_m.Called(dir, usefd)
}

// AllowMemory provides a mock function with given fields: size
func (_m *LibP2PMetrics) AllowMemory(size int) {
	_m.Called(size)
}

// AllowPeer provides a mock function with given fields: p
func (_m *LibP2PMetrics) AllowPeer(p peer.ID) {
	_m.Called(p)
}

// AllowProtocol provides a mock function with given fields: proto
func (_m *LibP2PMetrics) AllowProtocol(proto protocol.ID) {
	_m.Called(proto)
}

// AllowService provides a mock function with given fields: svc
func (_m *LibP2PMetrics) AllowService(svc string) {
	_m.Called(svc)
}

// AllowStream provides a mock function with given fields: p, dir
func (_m *LibP2PMetrics) AllowStream(p peer.ID, dir network.Direction) {
	_m.Called(p, dir)
}

// BlockConn provides a mock function with given fields: dir, usefd
func (_m *LibP2PMetrics) BlockConn(dir network.Direction, usefd bool) {
	_m.Called(dir, usefd)
}

// BlockMemory provides a mock function with given fields: size
func (_m *LibP2PMetrics) BlockMemory(size int) {
	_m.Called(size)
}

// BlockPeer provides a mock function with given fields: p
func (_m *LibP2PMetrics) BlockPeer(p peer.ID) {
	_m.Called(p)
}

// BlockProtocol provides a mock function with given fields: proto
func (_m *LibP2PMetrics) BlockProtocol(proto protocol.ID) {
	_m.Called(proto)
}

// BlockProtocolPeer provides a mock function with given fields: proto, p
func (_m *LibP2PMetrics) BlockProtocolPeer(proto protocol.ID, p peer.ID) {
	_m.Called(proto, p)
}

// BlockService provides a mock function with given fields: svc
func (_m *LibP2PMetrics) BlockService(svc string) {
	_m.Called(svc)
}

// BlockServicePeer provides a mock function with given fields: svc, p
func (_m *LibP2PMetrics) BlockServicePeer(svc string, p peer.ID) {
	_m.Called(svc, p)
}

// BlockStream provides a mock function with given fields: p, dir
func (_m *LibP2PMetrics) BlockStream(p peer.ID, dir network.Direction) {
	_m.Called(p, dir)
}

// DNSLookupDuration provides a mock function with given fields: duration
func (_m *LibP2PMetrics) DNSLookupDuration(duration time.Duration) {
	_m.Called(duration)
}

// InboundConnections provides a mock function with given fields: connectionCount
func (_m *LibP2PMetrics) InboundConnections(connectionCount uint) {
	_m.Called(connectionCount)
}

// OnAppSpecificScoreUpdated provides a mock function with given fields: _a0
func (_m *LibP2PMetrics) OnAppSpecificScoreUpdated(_a0 float64) {
	_m.Called(_a0)
}

// OnBehaviourPenaltyUpdated provides a mock function with given fields: _a0
func (_m *LibP2PMetrics) OnBehaviourPenaltyUpdated(_a0 float64) {
	_m.Called(_a0)
}

// OnDNSCacheHit provides a mock function with given fields:
func (_m *LibP2PMetrics) OnDNSCacheHit() {
	_m.Called()
}

// OnDNSCacheInvalidated provides a mock function with given fields:
func (_m *LibP2PMetrics) OnDNSCacheInvalidated() {
	_m.Called()
}

// OnDNSCacheMiss provides a mock function with given fields:
func (_m *LibP2PMetrics) OnDNSCacheMiss() {
	_m.Called()
}

// OnDNSLookupRequestDropped provides a mock function with given fields:
func (_m *LibP2PMetrics) OnDNSLookupRequestDropped() {
	_m.Called()
}

// OnFirstMessageDeliveredUpdated provides a mock function with given fields: _a0, _a1
func (_m *LibP2PMetrics) OnFirstMessageDeliveredUpdated(_a0 channels.Topic, _a1 float64) {
	_m.Called(_a0, _a1)
}

// OnGraftReceived provides a mock function with given fields: count
func (_m *LibP2PMetrics) OnGraftReceived(count int) {
	_m.Called(count)
}

// OnIHaveReceived provides a mock function with given fields: count
func (_m *LibP2PMetrics) OnIHaveReceived(count int) {
	_m.Called(count)
}

// OnIPColocationFactorUpdated provides a mock function with given fields: _a0
func (_m *LibP2PMetrics) OnIPColocationFactorUpdated(_a0 float64) {
	_m.Called(_a0)
}

// OnIWantReceived provides a mock function with given fields: count
func (_m *LibP2PMetrics) OnIWantReceived(count int) {
	_m.Called(count)
}

// OnIncomingRpcAcceptedFully provides a mock function with given fields:
func (_m *LibP2PMetrics) OnIncomingRpcAcceptedFully() {
	_m.Called()
}

// OnIncomingRpcAcceptedOnlyForControlMessages provides a mock function with given fields:
func (_m *LibP2PMetrics) OnIncomingRpcAcceptedOnlyForControlMessages() {
	_m.Called()
}

// OnIncomingRpcRejected provides a mock function with given fields:
func (_m *LibP2PMetrics) OnIncomingRpcRejected() {
	_m.Called()
}

// OnInvalidMessageDeliveredUpdated provides a mock function with given fields: _a0, _a1
func (_m *LibP2PMetrics) OnInvalidMessageDeliveredUpdated(_a0 channels.Topic, _a1 float64) {
	_m.Called(_a0, _a1)
}

// OnLocalMeshSizeUpdated provides a mock function with given fields: topic, size
func (_m *LibP2PMetrics) OnLocalMeshSizeUpdated(topic string, size int) {
	_m.Called(topic, size)
}

// OnMeshMessageDeliveredUpdated provides a mock function with given fields: _a0, _a1
func (_m *LibP2PMetrics) OnMeshMessageDeliveredUpdated(_a0 channels.Topic, _a1 float64) {
	_m.Called(_a0, _a1)
}

// OnOverallPeerScoreUpdated provides a mock function with given fields: _a0
func (_m *LibP2PMetrics) OnOverallPeerScoreUpdated(_a0 float64) {
	_m.Called(_a0)
}

// OnPruneReceived provides a mock function with given fields: count
func (_m *LibP2PMetrics) OnPruneReceived(count int) {
	_m.Called(count)
}

// OnPublishedGossipMessagesReceived provides a mock function with given fields: count
func (_m *LibP2PMetrics) OnPublishedGossipMessagesReceived(count int) {
	_m.Called(count)
}

// OnTimeInMeshUpdated provides a mock function with given fields: _a0, _a1
func (_m *LibP2PMetrics) OnTimeInMeshUpdated(_a0 channels.Topic, _a1 time.Duration) {
	_m.Called(_a0, _a1)
}

// OutboundConnections provides a mock function with given fields: connectionCount
func (_m *LibP2PMetrics) OutboundConnections(connectionCount uint) {
	_m.Called(connectionCount)
}

// RoutingTablePeerAdded provides a mock function with given fields:
func (_m *LibP2PMetrics) RoutingTablePeerAdded() {
	_m.Called()
}

// RoutingTablePeerRemoved provides a mock function with given fields:
func (_m *LibP2PMetrics) RoutingTablePeerRemoved() {
	_m.Called()
}

// SetWarningStateCount provides a mock function with given fields: _a0
func (_m *LibP2PMetrics) SetWarningStateCount(_a0 uint) {
	_m.Called(_a0)
}

type mockConstructorTestingTNewLibP2PMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewLibP2PMetrics creates a new instance of LibP2PMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewLibP2PMetrics(t mockConstructorTestingTNewLibP2PMetrics) *LibP2PMetrics {
	mock := &LibP2PMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

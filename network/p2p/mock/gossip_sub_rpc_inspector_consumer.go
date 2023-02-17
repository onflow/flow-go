// Code generated by mockery v2.13.1. DO NOT EDIT.

package mockp2p

import (
	p2p "github.com/onflow/flow-go/network/p2p"
	mock "github.com/stretchr/testify/mock"
)

// GossipSubRpcInspectorConsumer is an autogenerated mock type for the GossipSubRpcInspectorConsumer type
type GossipSubRpcInspectorConsumer struct {
	mock.Mock
}

// OnInvalidControlMessage provides a mock function with given fields: _a0
func (_m *GossipSubRpcInspectorConsumer) OnInvalidControlMessage(_a0 p2p.InvalidControlMessageNotification) {
	_m.Called(_a0)
}

type mockConstructorTestingTNewGossipSubRpcInspectorConsumer interface {
	mock.TestingT
	Cleanup(func())
}

// NewGossipSubRpcInspectorConsumer creates a new instance of GossipSubRpcInspectorConsumer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewGossipSubRpcInspectorConsumer(t mockConstructorTestingTNewGossipSubRpcInspectorConsumer) *GossipSubRpcInspectorConsumer {
	mock := &GossipSubRpcInspectorConsumer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

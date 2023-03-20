// Code generated by mockery v2.21.4. DO NOT EDIT.

package mockp2p

import (
	channels "github.com/onflow/flow-go/network/channels"
	mock "github.com/stretchr/testify/mock"

	peer "github.com/libp2p/go-libp2p/core/peer"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// PeerScoringBuilder is an autogenerated mock type for the PeerScoringBuilder type
type PeerScoringBuilder struct {
	mock.Mock
}

// SetAppSpecificScoreParams provides a mock function with given fields: _a0
func (_m *PeerScoringBuilder) SetAppSpecificScoreParams(_a0 func(peer.ID) float64) {
	_m.Called(_a0)
}

// SetTopicScoreParams provides a mock function with given fields: topic, topicScoreParams
func (_m *PeerScoringBuilder) SetTopicScoreParams(topic channels.Topic, topicScoreParams *pubsub.TopicScoreParams) {
	_m.Called(topic, topicScoreParams)
}

type mockConstructorTestingTNewPeerScoringBuilder interface {
	mock.TestingT
	Cleanup(func())
}

// NewPeerScoringBuilder creates a new instance of PeerScoringBuilder. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewPeerScoringBuilder(t mockConstructorTestingTNewPeerScoringBuilder) *PeerScoringBuilder {
	mock := &PeerScoringBuilder{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

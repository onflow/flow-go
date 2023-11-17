package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// MockUpdatableTopicProvider is a mock implementation of the TopicProvider interface.
// TODO: this should be moved to a common package (e.g. network/p2p/test). Currently, it is not possible to do so because of a circular dependency.
type MockUpdatableTopicProvider struct {
	topics        []string
	subscriptions map[string][]peer.ID
}

func NewMockUpdatableTopicProvider() *MockUpdatableTopicProvider {
	return &MockUpdatableTopicProvider{
		topics:        []string{},
		subscriptions: map[string][]peer.ID{},
	}
}

func (m *MockUpdatableTopicProvider) GetTopics() []string {
	return m.topics
}

func (m *MockUpdatableTopicProvider) ListPeers(topic string) []peer.ID {
	return m.subscriptions[topic]
}

func (m *MockUpdatableTopicProvider) UpdateTopics(topics []string) {
	m.topics = topics
}

func (m *MockUpdatableTopicProvider) UpdateSubscriptions(topic string, peers []peer.ID) {
	m.subscriptions[topic] = peers
}

package p2p

import "github.com/libp2p/go-libp2p/core/peer"

// SubscriptionProvider provides a list of topics a peer is subscribed to.
type SubscriptionProvider interface {
	// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
	GetSubscribedTopics(pid peer.ID) []string
}

// TopicProvider provides a low-level abstraction for pubsub to
// perform topic-related queries.
type TopicProvider interface {
	// GetTopics returns all the topics within the pubsub network.
	GetTopics() []string

	// ListPeers returns all the peers subscribed to a topic.
	ListPeers(topic string) []peer.ID
}

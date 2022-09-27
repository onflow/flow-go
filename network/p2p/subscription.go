package p2p

import "github.com/libp2p/go-libp2p/core/peer"

// SubscriptionProvider provides a list of topics a peer is subscribed to.
type SubscriptionProvider interface {
	// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
	// Note that the current peer must be subscribed to the topic for it to the same topics in order
	// to query for other peers, e.g., if current peer has subscribed to topics A and B, and peer1
	// has subscribed to topics A, B, and C, then GetSubscribedTopics(peer1) will return A and B. Since this peer
	// has not subscribed to topic C, it will not be able to query for other peers subscribed to topic C.
	GetSubscribedTopics(pid peer.ID) []string
}

// TopicProvider provides a low-level abstraction for pubsub to perform topic-related queries.
// This abstraction is provided to encapsulate the pubsub implementation details from the rest of the codebase.
type TopicProvider interface {
	// GetTopics returns all the topics within the pubsub network that the current peer has subscribed to.
	GetTopics() []string

	// ListPeers returns all the peers subscribed to a topic.
	// Note that the current peer must be subscribed to the topic for it to query for other peers.
	// If the current peer is not subscribed to the topic, an empty list is returned.
	// For example, if current peer has subscribed to topics A and B, then ListPeers only return
	// subscribed peers for topics A and B, and querying for topic C will return an empty list.
	ListPeers(topic string) []peer.ID
}

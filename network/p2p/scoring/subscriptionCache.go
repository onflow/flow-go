package scoring

import "github.com/libp2p/go-libp2p/core/peer"

type SubscriptionCache interface {
	// GetSubscribedTopics returns the list of topics a peer is subscribed to.
	GetSubscribedTopics(id peer.ID) []string

	// ClearTopics clears the list of topics a peer is subscribed to if the peer is in the cache.
	IncrementUpdateCycle()

	// UpdateTopicForPeer updates the list of topics a peer is subscribed to by appending the given topic to the list.
	// The boolean return value indicates whether the topic was added to the list of topics for the peer.
	// Note that depending on the ejection
	AddTopicForPeer(id peer.ID, topic string) bool
}

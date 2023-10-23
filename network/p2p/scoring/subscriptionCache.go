package scoring

import "github.com/libp2p/go-libp2p/core/peer"

type SubscriptionCache interface {
	// GetSubscribedTopics returns the list of topics a peer is subscribed to.
	GetSubscribedTopics(id peer.ID) []string

	// MoveToNextUpdateCycle let's the cache know that a new update cycle has started.
	// This is used to clean up old topic subscriptions of peers upon update.
	MoveToNextUpdateCycle()

	// AddTopicForPeer appends the topic to the list of topics a peer is subscribed to for the current update cycle.
	AddTopicForPeer(id peer.ID, topic string) bool
}

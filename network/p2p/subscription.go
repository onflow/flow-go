package p2p

import "github.com/libp2p/go-libp2p/core/peer"

// SubscriptionProvider provides a list of topics a peer is subscribed to.
type SubscriptionProvider interface {
	// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
	GetSubscribedTopics(pid peer.ID) []string
}

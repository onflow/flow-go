package scoring

import "github.com/libp2p/go-libp2p/core/peer"

// SubscriptionCache implements an in-memory cache that keeps track of the topics a peer is subscribed to.
// The cache is modeled abstracted to be used in update cycles, i.e., every regular interval of time, the cache is updated for
// all peers.
type SubscriptionCache interface {
	// GetSubscribedTopics returns the list of topics a peer is subscribed to.
	// Returns:
	// - []string: the list of topics the peer is subscribed to.
	// - bool: true if there is a record for the peer, false otherwise.
	GetSubscribedTopics(pid peer.ID) ([]string, bool)

	// MoveToNextUpdateCycle moves the subscription cache to the next update cycle.
	// A new update cycle is started when the subscription cache is first created, and then every time the subscription cache
	// is updated. The update cycle is used to keep track of the last time the subscription cache was updated. It is used to
	// implement a notion of time in the subscription cache.
	// Returns:
	// - uint64: the current update cycle.
	MoveToNextUpdateCycle() uint64

	// AddTopicForPeer appends a topic to the list of topics a peer is subscribed to. If the peer is not subscribed to any
	// topics yet, a new record is created.
	// If the last update cycle is older than the current cycle, the list of topics for the peer is first cleared, and then
	// the topic is added to the list. This is to ensure that the list of topics for a peer is always up to date.
	// Args:
	// - pid: the peer id of the peer.
	// - topic: the topic to add.
	// Returns:
	// - []string: the list of topics the peer is subscribed to after the update.
	// - error: an error if the update failed; any returned error is an irrecoverable error and indicates a bug or misconfiguration.
	// Implementation must be thread-safe.
	AddWithInitTopicForPeer(pid peer.ID, topic string) ([]string, error)
}

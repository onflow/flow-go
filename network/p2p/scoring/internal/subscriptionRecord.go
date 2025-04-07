package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// SubscriptionRecord represents the list of topics a peer is subscribed to.
// It is internally used by the SubscriptionRecordCache to store the subscription records in the cache.
type SubscriptionRecord struct {
	// PeerID is the peer id of the peer that is the owner of the subscription.
	PeerID peer.ID

	// Topics is the list of topics the peer is subscribed to.
	Topics []string

	// LastUpdatedCycle is the last cycle counter value that this record was updated.
	// This is used to clean up old records' topics upon update.
	LastUpdatedCycle uint64
}

package cache

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// ClusterPrefixedMessagesReceivedRecord cache record that keeps track of the amount of cluster prefixed control messages received from a peer.
// This struct implements the flow.Entity interface and uses  node ID of the sender for deduplication.
type ClusterPrefixedMessagesReceivedRecord struct {
	// NodeID the node ID of the sender.
	NodeID flow.Identifier
	// Gauge represents the approximate amount of cluster prefixed messages received by a peer, this
	// value is decayed back to 0 after some time.
	Gauge       float64
	lastUpdated time.Time
}

func NewClusterPrefixedMessagesReceivedRecord(nodeID flow.Identifier) ClusterPrefixedMessagesReceivedRecord {
	return ClusterPrefixedMessagesReceivedRecord{
		NodeID:      nodeID,
		Gauge:       0.0,
		lastUpdated: time.Now(),
	}
}

var _ flow.Entity = (*ClusterPrefixedMessagesReceivedRecord)(nil)

// ID returns the node ID of the sender, which is used as the unique identifier of the entity for maintenance and
// deduplication purposes in the cache.
func (c ClusterPrefixedMessagesReceivedRecord) ID() flow.Identifier {
	return c.NodeID
}

// Checksum returns the node ID of the sender, it does not have any purpose in the cache.
// It is implemented to satisfy the flow.Entity interface.
func (c ClusterPrefixedMessagesReceivedRecord) Checksum() flow.Identifier {
	return c.NodeID
}

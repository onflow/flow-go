package cache

import (
	"time"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
)

// ClusterPrefixedMessagesReceivedRecord cache record that keeps track of the amount of cluster prefixed control messages received from a peer.
// This struct implements the flow.Entity interface and uses  node ID of the sender for deduplication.
type ClusterPrefixedMessagesReceivedRecord struct {
	NodeID      flow.Identifier
	Counter     *atomic.Float64
	lastUpdated time.Time
}

func NewClusterPrefixedMessagesReceivedRecord(nodeID flow.Identifier) ClusterPrefixedMessagesReceivedRecord {
	return ClusterPrefixedMessagesReceivedRecord{
		NodeID:      nodeID,
		Counter:     atomic.NewFloat64(0),
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

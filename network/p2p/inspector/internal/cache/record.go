package cache

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
)

// ClusterPrefixedMessagesReceivedRecord cache record that keeps track of the amount of cluster prefixed control messages received from a peer.
type ClusterPrefixedMessagesReceivedRecord struct {
	NodeID  flow.Identifier
	Counter *atomic.Float64
}

func NewClusterPrefixedMessagesReceivedRecord(nodeID flow.Identifier) ClusterPrefixedMessagesReceivedRecord {
	return ClusterPrefixedMessagesReceivedRecord{
		NodeID:  nodeID,
		Counter: atomic.NewFloat64(0),
	}
}

package cache

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
)

// ClusterPrefixTopicsReceivedRecord cache record that keeps track of the amount of cluster prefixed
// topics received from a peer.
type ClusterPrefixTopicsReceivedRecord struct {
	Identifier flow.Identifier
	Counter    *atomic.Int64
}

func NewClusterPrefixTopicsReceivedRecord(identifier flow.Identifier) ClusterPrefixTopicsReceivedRecord {
	return ClusterPrefixTopicsReceivedRecord{
		Identifier: identifier,
		Counter:    atomic.NewInt64(0),
	}
}

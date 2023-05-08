package cache

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/metrics"
)

// ClusterPrefixTopicsReceivedTracker struct that keeps track of the amount of cluster prefixed control messages received by a peer.
type ClusterPrefixTopicsReceivedTracker struct {
	cache *RecordCache
}

// NewClusterPrefixTopicsReceivedTracker returns a new *ClusterPrefixTopicsReceivedTracker.
func NewClusterPrefixTopicsReceivedTracker(logger zerolog.Logger, sizeLimit uint32, cacheOpts ...RecordCacheConfigOpt) *ClusterPrefixTopicsReceivedTracker {
	config := &RecordCacheConfig{
		sizeLimit: sizeLimit,
		logger:    logger,
		collector: metrics.NewNoopCollector(),
	}

	for _, opt := range cacheOpts {
		opt(config)
	}

	return &ClusterPrefixTopicsReceivedTracker{cache: NewRecordCache(config, NewRecordEntity)}
}

// Inc increments the cluster prefixed topics received Counter for the peer.
func (c *ClusterPrefixTopicsReceivedTracker) Inc(peerID peer.ID) (int64, error) {
	count, err := c.cache.Update(peerID)
	if err != nil {
		return 0, fmt.Errorf("failed to increment cluster prefixed received tracker Counter for peer %s: %w", peerID, err)
	}
	return count, nil
}

// Load loads the current number of cluster prefixed topics received by a peer.
func (c *ClusterPrefixTopicsReceivedTracker) Load(peerID peer.ID) int64 {
	count, _ := c.cache.Get(peerID)
	return count
}

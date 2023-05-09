package cache

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// ClusterPrefixTopicsReceivedTracker struct that keeps track of the amount of cluster prefixed control messages received by a peer.
type ClusterPrefixTopicsReceivedTracker struct {
	cache *RecordCache
}

// NewClusterPrefixTopicsReceivedTracker returns a new *ClusterPrefixTopicsReceivedTracker.
func NewClusterPrefixTopicsReceivedTracker(logger zerolog.Logger, sizeLimit uint32, clusterPrefixedCacheCollector module.HeroCacheMetrics, decay float64) *ClusterPrefixTopicsReceivedTracker {
	config := &RecordCacheConfig{
		sizeLimit:   sizeLimit,
		logger:      logger,
		collector:   clusterPrefixedCacheCollector,
		recordDecay: decay,
	}
	return &ClusterPrefixTopicsReceivedTracker{cache: NewRecordCache(config, NewRecordEntity)}
}

// Inc increments the cluster prefixed topics received Counter for the peer.
func (c *ClusterPrefixTopicsReceivedTracker) Inc(id flow.Identifier) (float64, error) {
	count, err := c.cache.Update(id)
	if err != nil {
		return 0, fmt.Errorf("failed to increment cluster prefixed received tracker Counter for peer %s: %w", id, err)
	}
	return count, nil
}

// Load loads the current number of cluster prefixed topics received by a peer.
func (c *ClusterPrefixTopicsReceivedTracker) Load(id flow.Identifier) float64 {
	count, _, _ := c.cache.Get(id)
	return count
}

// StoreActiveClusterIds stores the active cluster Ids in the underlying record cache.
func (c *ClusterPrefixTopicsReceivedTracker) StoreActiveClusterIds(clusterIdList flow.ChainIDList) {
	c.cache.storeActiveClusterIds(clusterIdList)
}

// GetActiveClusterIds gets the active cluster Ids from the underlying record cache.
func (c *ClusterPrefixTopicsReceivedTracker) GetActiveClusterIds() flow.ChainIDList {
	return c.cache.getActiveClusterIds()
}

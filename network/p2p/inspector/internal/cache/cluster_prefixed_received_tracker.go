package cache

import (
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// ClusterPrefixedMessagesReceivedTracker struct that keeps track of the amount of cluster prefixed control messages received by a peer.
type ClusterPrefixedMessagesReceivedTracker struct {
	cache *RecordCache
	// activeClusterIds atomic pointer that stores the current active cluster IDs. This ensures safe concurrent access to the activeClusterIds internal flow.ChainIDList.
	activeClusterIds *atomic.Pointer[flow.ChainIDList]
}

// NewClusterPrefixedMessagesReceivedTracker returns a new *ClusterPrefixedMessagesReceivedTracker.
func NewClusterPrefixedMessagesReceivedTracker(logger zerolog.Logger, sizeLimit uint32, clusterPrefixedCacheCollector module.HeroCacheMetrics, decay float64) (*ClusterPrefixedMessagesReceivedTracker,
	error) {
	config := &RecordCacheConfig{
		sizeLimit:   sizeLimit,
		logger:      logger,
		collector:   clusterPrefixedCacheCollector,
		recordDecay: decay,
	}
	recordCache, err := NewRecordCache(config, NewClusterPrefixedMessagesReceivedRecord)
	if err != nil {
		return nil, fmt.Errorf("failed to create new record cahe: %w", err)
	}
	return &ClusterPrefixedMessagesReceivedTracker{cache: recordCache, activeClusterIds: atomic.NewPointer[flow.ChainIDList](&flow.ChainIDList{})}, nil
}

// Inc increments the cluster prefixed control messages received Gauge for the peer.
// All errors returned from this func are unexpected and irrecoverable.
func (c *ClusterPrefixedMessagesReceivedTracker) Inc(nodeID flow.Identifier) (float64, error) {
	count, err := c.cache.ReceivedClusterPrefixedMessage(nodeID)
	if err != nil {
		return 0, fmt.Errorf("failed to increment cluster prefixed received tracker gauge value for peer %s: %w", nodeID, err)
	}
	return count, nil
}

// Load loads the current number of cluster prefixed control messages received by a peer.
// All errors returned from this func are unexpected and irrecoverable.
func (c *ClusterPrefixedMessagesReceivedTracker) Load(nodeID flow.Identifier) (float64, error) {
	count, _, err := c.cache.GetWithInit(nodeID)
	if err != nil {
		return 0, fmt.Errorf("failed to get cluster prefixed received tracker gauge value for peer %s: %w", nodeID, err)
	}
	return count, nil
}

// StoreActiveClusterIds stores the active cluster Ids in the underlying record cache.
func (c *ClusterPrefixedMessagesReceivedTracker) StoreActiveClusterIds(clusterIdList flow.ChainIDList) {
	c.activeClusterIds.Store(&clusterIdList)
}

// GetActiveClusterIds gets the active cluster Ids from the underlying record cache.
func (c *ClusterPrefixedMessagesReceivedTracker) GetActiveClusterIds() flow.ChainIDList {
	return *c.activeClusterIds.Load()
}

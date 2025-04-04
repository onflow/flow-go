package cache

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/scoring"
)

type recordFactory func(nodeID flow.Identifier) *ClusterPrefixedMessagesReceivedRecord

type RecordCacheConfig struct {
	sizeLimit uint32
	logger    zerolog.Logger
	collector module.HeroCacheMetrics
	// recordDecay decay factor used by the cache to perform geometric decay on gauge values.
	recordDecay float64
}

// RecordCache is a cache that stores ClusterPrefixedMessagesReceivedRecord by peer node ID. Each record
// contains a float64 Gauge field that indicates the current approximate number cluster prefixed control messages that were allowed to bypass
// validation due to some error that will prevent the message from being validated.
// Each record contains a float64 Gauge field that is decayed overtime back to 0. This ensures that nodes that fall
// behind in the protocol can catch up.
type RecordCache struct {
	// recordFactory is a factory function that creates a new ClusterPrefixedMessagesReceivedRecord.
	recordFactory recordFactory
	// c is the underlying cache.
	c *stdmap.Backend[flow.Identifier, *ClusterPrefixedMessagesReceivedRecord]
	// decayFunc decay func used by the cache to perform decay on gauges.
	decayFunc decayFunc
}

// NewRecordCache creates a new *RecordCache.
// Args:
// - config: record cache config.
// - recordFactory: a factory function that creates a new spam record.
// Returns:
// - *RecordCache, the created cache.
// Note that this cache is supposed to keep the cluster prefix control messages received record for the authorized (staked) nodes. Since the number of such nodes is
// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
// the records of the authorized nodes. Also, this cache is keeping at most one record per peer id, so the
// size of the cache must be at least the number of authorized nodes.
func NewRecordCache(config *RecordCacheConfig, recordFactory recordFactory) (*RecordCache, error) {
	backData := herocache.NewCache[*ClusterPrefixedMessagesReceivedRecord](
		config.sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		config.logger.With().Str("mempool", "gossipsub=cluster-prefix-control-messages-received-records").Logger(),
		config.collector,
	)
	return &RecordCache{
		recordFactory: recordFactory,
		decayFunc:     defaultDecayFunction(config.recordDecay),
		c:             stdmap.NewBackend(stdmap.WithMutableBackData[flow.Identifier, *ClusterPrefixedMessagesReceivedRecord](backData)),
	}, nil
}

// ReceivedClusterPrefixedMessage applies an adjustment that increments the number of cluster prefixed control messages received by a peer.
// Returns number of cluster prefix control messages received after the adjustment. The record is initialized before
// the adjustment func is applied that will increment the Gauge.
// Args:
// - pid: the peer ID of the sender of the control message.
// Returns:
//   - The cluster prefix control messages received gauge value after the adjustment.
//   - exception only in cases of internal data inconsistency or bugs. No errors are expected.
func (r *RecordCache) ReceivedClusterPrefixedMessage(pid peer.ID) (float64, error) {
	var err error
	adjustFunc := func(record *ClusterPrefixedMessagesReceivedRecord) *ClusterPrefixedMessagesReceivedRecord {
		record, err = r.decayAdjustment(record) // first decay the record
		if err != nil {
			return record
		}
		return r.incrementAdjustment(record) // then increment the record
	}
	nodeID := p2p.MakeId(pid)
	adjustedRecord, adjusted := r.c.AdjustWithInit(nodeID, adjustFunc, func() *ClusterPrefixedMessagesReceivedRecord {
		return r.recordFactory(nodeID)
	})

	if err != nil {
		return 0, fmt.Errorf("unexpected error while applying decay and increment adjustments for peer %s: %w", pid, err)
	}

	if !adjusted {
		return 0, fmt.Errorf("adjustment failed for peer %s", pid)
	}

	return adjustedRecord.Gauge, nil
}

// GetWithInit returns the current number of cluster prefixed control messages received from a peer.
// The record is initialized before the count is returned.
// Before the control messages received gauge value is returned it is decayed using the configured decay function.
// Returns the record and true if the record exists, nil and false otherwise.
// Args:
// - pid: the peer ID of the sender of the control message.
// Returns:
// - The cluster prefixed control messages received gauge value after the decay and true if the record exists, 0 and false otherwise.
// No errors are expected during normal operation.
func (r *RecordCache) GetWithInit(pid peer.ID) (float64, bool, error) {
	var err error
	adjustLogic := func(record *ClusterPrefixedMessagesReceivedRecord) *ClusterPrefixedMessagesReceivedRecord {
		// perform decay on gauge value
		record, err = r.decayAdjustment(record)
		return record
	}
	nodeID := p2p.MakeId(pid)
	adjustedRecord, adjusted := r.c.AdjustWithInit(nodeID, adjustLogic, func() *ClusterPrefixedMessagesReceivedRecord {
		return r.recordFactory(nodeID)
	})
	if err != nil {
		return 0, false, fmt.Errorf("unexpected error while applying decay adjustment for peer %s: %w", pid, err)
	}
	if !adjusted {
		return 0, false, fmt.Errorf("decay adjustment failed for peer %s", pid)
	}

	return adjustedRecord.Gauge, true, nil
}

// Remove removes the record of the given peer id from the cache.
// Returns true if the record is removed, false otherwise (i.e., the record does not exist).
// Args:
// - pid: the peer ID of the sender of the control message.
// Returns:
// - true if the record is removed, false otherwise (i.e., the record does not exist).
func (r *RecordCache) Remove(pid peer.ID) bool {
	return r.c.Remove(p2p.MakeId(pid))
}

// NodeIDs returns the list of identities of the nodes that have a spam record in the cache.
func (r *RecordCache) NodeIDs() flow.IdentifierList {
	all := r.c.All()

	nodeIDs := make(flow.IdentifierList, 0, len(all))
	for nodeID := range all {
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs
}

// Size returns the number of records in the cache.
func (r *RecordCache) Size() uint {
	return r.c.Size()
}

func (r *RecordCache) incrementAdjustment(record *ClusterPrefixedMessagesReceivedRecord) *ClusterPrefixedMessagesReceivedRecord {
	record.Gauge++
	record.lastUpdated = time.Now()
	// Return the adjusted record.
	return record
}

// All errors returned from this function are unexpected and irrecoverable.
func (r *RecordCache) decayAdjustment(record *ClusterPrefixedMessagesReceivedRecord) (*ClusterPrefixedMessagesReceivedRecord, error) {
	var err error
	record, err = r.decayFunc(record)
	if err != nil {
		return record, err
	}
	record.lastUpdated = time.Now()
	// Return the adjusted record.
	return record, nil
}

// decayFunc the callback used to apply a decay method to the record.
// All errors returned from this callback are unexpected and irrecoverable.
type decayFunc func(record *ClusterPrefixedMessagesReceivedRecord) (*ClusterPrefixedMessagesReceivedRecord, error)

// defaultDecayFunction is the default decay function that is used to decay the cluster prefixed control message received gauge of a peer.
// All errors returned are unexpected and irrecoverable.
func defaultDecayFunction(decay float64) decayFunc {
	return func(record *ClusterPrefixedMessagesReceivedRecord) (*ClusterPrefixedMessagesReceivedRecord, error) {
		received := record.Gauge
		if received == 0 {
			return record, nil
		}

		decayedVal, err := scoring.GeometricDecay(received, decay, record.lastUpdated)
		if err != nil {
			return record, fmt.Errorf("could not decay cluster prefixed control messages received gauge: %w", err)
		}
		record.Gauge = decayedVal
		return record, nil
	}
}

package cache

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p/scoring"
)

type recordEntityFactory func(identifier flow.Identifier) ClusterPrefixedMessagesReceivedRecord

type RecordCacheConfig struct {
	sizeLimit uint32
	logger    zerolog.Logger
	collector module.HeroCacheMetrics
	// recordDecay decay factor used by the cache to perform geometric decay on gauge values.
	recordDecay float64
}

// RecordCache is a cache that stores *ClusterPrefixedMessagesReceivedRecord by peer node ID. Each record
// contains a float64 Gauge field that indicates the current number cluster prefixed control messages that were allowed to bypass
// validation due to the active cluster ids not being set or an unknown cluster ID error is encountered during validation.
// Each record contains a float64 Gauge field that is decayed overtime back to 0. This ensures that nodes that fall
// behind in the protocol can catch up.
type RecordCache struct {
	// recordEntityFactory is a factory function that creates a new *ClusterPrefixedMessagesReceivedRecord.
	recordEntityFactory recordEntityFactory
	// c is the underlying cache.
	c *stdmap.Backend
	// decayFunc decay func used by the cache to perform decay on gauges.
	decayFunc preProcessingFunc
}

// NewRecordCache creates a new *RecordCache.
// Args:
// - sizeLimit: the maximum number of records that the cache can hold.
// - logger: the logger used by the cache.
// - collector: the metrics collector used by the cache.
// - recordEntityFactory: a factory function that creates a new spam record.
// Returns:
// - *RecordCache, the created cache.
// Note that this cache is supposed to keep the cluster prefix control messages received record for the authorized (staked) nodes. Since the number of such nodes is
// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
// the records of the authorized nodes. Also, this cache is keeping at most one record per peer id, so the
// size of the cache must be at least the number of authorized nodes.
func NewRecordCache(config *RecordCacheConfig, recordEntityFactory recordEntityFactory) (*RecordCache, error) {
	backData := herocache.NewCache(config.sizeLimit,
		herocache.DefaultOversizeFactor,
		// this cache is supposed to keep the cluster prefix control messages received record for the authorized (staked) nodes. Since the number of such nodes is
		// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
		// the records of the authorized nodes. Also, this cache is keeping at most one record per peer id, so the
		// size of the cache must be at least the number of authorized nodes.
		heropool.NoEjection,
		config.logger.With().Str("mempool", "gossipsub=cluster-prefix-control-messages-received-records").Logger(),
		config.collector)
	return &RecordCache{
		recordEntityFactory: recordEntityFactory,
		decayFunc:           defaultDecayFunction(config.recordDecay),
		c:                   stdmap.NewBackend(stdmap.WithBackData(backData)),
	}, nil
}

// Init initializes the record cache for the given peer id if it does not exist.
// Returns true if the record is initialized, false otherwise (i.e.: the record already exists).
// Args:
// - nodeID: the node ID of the sender of the control message.
// Returns:
// - true if the record is initialized, false otherwise (i.e.: the record already exists).
// Note that if Init is called multiple times for the same peer id, the record is initialized only once, and the
// subsequent calls return false and do not change the record (i.e.: the record is not re-initialized).
func (r *RecordCache) Init(nodeID flow.Identifier) bool {
	entity := r.recordEntityFactory(nodeID)
	return r.c.Add(entity)
}

// Update applies an adjustment that increments the number of cluster prefixed control messages received by a peer.
// Returns number of cluster prefix control messages received after the adjustment. The record is initialized before
// the adjustment func is applied that will increment the Gauge.
// It returns an error if the adjustFunc returns an error or if the record does not exist.
// Assuming that adjust is always called when the record exists, the error is irrecoverable and indicates a bug.
// Args:
// - nodeID: the node ID of the sender of the control message.
// - adjustFunc: the function that adjusts the record.
// Returns:
//   - The number of cluster prefix control messages received after the adjustment.
//   - error if the adjustFunc returns an error or if the record does not exist (ErrRecordNotFound).
//     All errors should be treated as an irrecoverable error and indicates a bug.
//
// Note if Adjust is called under the assumption that the record exists, the ErrRecordNotFound should be treated
// as an irrecoverable error and indicates a bug.
func (r *RecordCache) Update(nodeID flow.Identifier) (float64, error) {
	optimisticAdjustFunc := func() (flow.Entity, bool) {
		return r.c.Adjust(nodeID, func(entity flow.Entity) flow.Entity {
			r.decayAdjustment(entity)            // first decay the record
			return r.incrementAdjustment(entity) // then increment the record
		})
	}

	// optimisticAdjustFunc is called assuming the record exists; if the record does not exist,
	// it means the record was not initialized. In this case, initialize the record and call optimisticAdjustFunc again.
	// If the record was initialized, optimisticAdjustFunc will be called only once.
	adjustedEntity, adjusted := optimisticAdjustFunc()
	if !adjusted {
		r.Init(nodeID)
		adjustedEntity, adjusted = optimisticAdjustFunc()
		if !adjusted {
			return 0, fmt.Errorf("unexpected record not found for node ID %s, even after an init attempt", nodeID)
		}
	}

	return adjustedEntity.(ClusterPrefixedMessagesReceivedRecord).Gauge.Load(), nil
}

// Get returns the current number of cluster prefixed control messages received from a peer.
// The record is initialized before the count is returned.
// Before the count is returned it is decayed using the configured decay function.
// Returns the record and true if the record exists, nil and false otherwise.
// Args:
// - nodeID: the node ID of the sender of the control message.
// Returns:
// - The number of cluster prefixed control messages received after the decay and true if the record exists, 0 and false otherwise.
func (r *RecordCache) Get(nodeID flow.Identifier) (float64, bool, error) {
	if r.Init(nodeID) {
		return 0, true, nil
	}

	adjustedEntity, adjusted := r.c.Adjust(nodeID, r.decayAdjustment)
	if !adjusted {
		return 0, false, fmt.Errorf("unexpected record not found for node ID %s, even after an init attempt", nodeID)
	}

	record, ok := adjustedEntity.(ClusterPrefixedMessagesReceivedRecord)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains ClusterPrefixedMessagesReceivedRecord entities.
		panic(fmt.Sprintf("invalid entity type, expected ClusterPrefixedMessagesReceivedRecord type, got: %T", adjustedEntity))
	}

	// perform decay on Gauge
	return record.Gauge.Load(), true, nil
}

// Identities returns the list of identities of the nodes that have a spam record in the cache.
func (r *RecordCache) Identities() []flow.Identifier {
	return flow.GetIDs(r.c.All())
}

// Remove removes the record of the given peer id from the cache.
// Returns true if the record is removed, false otherwise (i.e., the record does not exist).
// Args:
// - nodeID: the node ID of the sender of the control message.
// Returns:
// - true if the record is removed, false otherwise (i.e., the record does not exist).
func (r *RecordCache) Remove(nodeID flow.Identifier) bool {
	return r.c.Remove(nodeID)
}

// Size returns the number of records in the cache.
func (r *RecordCache) Size() uint {
	return r.c.Size()
}

func (r *RecordCache) incrementAdjustment(entity flow.Entity) flow.Entity {
	record, ok := entity.(ClusterPrefixedMessagesReceivedRecord)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains ClusterPrefixedMessagesReceivedRecord entities.
		panic(fmt.Sprintf("invalid entity type, expected ClusterPrefixedMessagesReceivedRecord type, got: %T", entity))
	}
	record.Gauge.Add(1)
	record.lastUpdated = time.Now()
	// Return the adjusted record.
	return record
}

func (r *RecordCache) decayAdjustment(entity flow.Entity) flow.Entity {
	record, ok := entity.(ClusterPrefixedMessagesReceivedRecord)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains ClusterPrefixedMessagesReceivedRecord entities.
		panic(fmt.Sprintf("invalid entity type, expected ClusterPrefixedMessagesReceivedRecord type, got: %T", entity))
	}
	var err error
	record, err = r.decayFunc(record)
	if err != nil {
		return record
	}
	record.lastUpdated = time.Now()
	// Return the adjusted record.
	return record
}

type preProcessingFunc func(recordEntity ClusterPrefixedMessagesReceivedRecord) (ClusterPrefixedMessagesReceivedRecord, error)

// defaultDecayFunction is the default decay function that is used to decay the cluster prefixed control message received gauge of a peer.
func defaultDecayFunction(decay float64) preProcessingFunc {
	return func(recordEntity ClusterPrefixedMessagesReceivedRecord) (ClusterPrefixedMessagesReceivedRecord, error) {
		received := recordEntity.Gauge.Load()
		if received == 0 {
			return recordEntity, nil
		}

		decayedVal, err := scoring.GeometricDecay(received, decay, recordEntity.lastUpdated)
		if err != nil {
			return recordEntity, fmt.Errorf("could not decay cluster prefixed control messages received gauge: %w", err)
		}
		recordEntity.Gauge.Store(decayedVal)
		return recordEntity, nil
	}
}

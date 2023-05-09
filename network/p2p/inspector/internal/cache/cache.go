package cache

import (
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

var ErrRecordNotFound = fmt.Errorf("record not found")

type recordEntityFactory func(identifier flow.Identifier) RecordEntity

type RecordCacheConfig struct {
	sizeLimit uint32
	logger    zerolog.Logger
	collector module.HeroCacheMetrics
}

// RecordCache is a cache that stores *ClusterPrefixTopicsReceivedRecord used by the control message validation inspector
// to keep track of the amount of cluster prefixed control messages received by a peer.
type RecordCache struct {
	// recordEntityFactory is a factory function that creates a new *RecordEntity.
	recordEntityFactory recordEntityFactory
	// c is the underlying cache.
	c *stdmap.Backend
}

// NewRecordCache creates a new *RecordCache.
// Args:
// - sizeLimit: the maximum number of records that the cache can hold.
// - logger: the logger used by the cache.
// - collector: the metrics collector used by the cache.
// - recordEntityFactory: a factory function that creates a new spam record.
// Returns:
// - *RecordCache, the created cache.
// Note that this cache is supposed to keep the cluster prefix topics received record for the authorized (staked) nodes. Since the number of such nodes is
// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
// the records of the authorized nodes. Also, this cache is keeping at most one record per peer id, so the
// size of the cache must be at least the number of authorized nodes.
func NewRecordCache(config *RecordCacheConfig, recordEntityFactory recordEntityFactory) *RecordCache {
	backData := herocache.NewCache(config.sizeLimit,
		herocache.DefaultOversizeFactor,
		// this cache is supposed to keep the cluster prefix topics received record for the authorized (staked) nodes. Since the number of such nodes is
		// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
		// the records of the authorized nodes. Also, this cache is keeping at most one record per peer id, so the
		// size of the cache must be at least the number of authorized nodes.
		heropool.NoEjection,
		config.logger.With().Str("mempool", "gossipsub=cluster-prefix-topics-received-records").Logger(),
		config.collector)

	return &RecordCache{
		recordEntityFactory: recordEntityFactory,
		c:                   stdmap.NewBackend(stdmap.WithBackData(backData)),
	}
}

// Init initializes the record cache for the given peer id if it does not exist.
// Returns true if the record is initialized, false otherwise (i.e.: the record already exists).
// Args:
// - originId: the origin id the sender of the control message.
// Returns:
// - true if the record is initialized, false otherwise (i.e.: the record already exists).
// Note that if Init is called multiple times for the same peer id, the record is initialized only once, and the
// subsequent calls return false and do not change the record (i.e.: the record is not re-initialized).
func (r *RecordCache) Init(originId flow.Identifier) bool {
	entity := r.recordEntityFactory(originId)
	return r.c.Add(entity)
}

// Update applies an adjustment that increments the number of cluster prefixed topics received by a peer.
// Returns number of cluster prefix topics received after the adjustment. The record is initialized before
// the adjustment func is applied that will increment the Counter.
// It returns an error if the adjustFunc returns an error or if the record does not exist.
// Assuming that adjust is always called when the record exists, the error is irrecoverable and indicates a bug.
// Args:
// - originId: the origin id the sender of the control message.
// - adjustFunc: the function that adjusts the record.
// Returns:
//   - The number of cluster prefix topics received after the adjustment.
//   - error if the adjustFunc returns an error or if the record does not exist (ErrRecordNotFound).
//     All errors should be treated as an irrecoverable error and indicates a bug.
//
// Note if Adjust is called under the assumption that the record exists, the ErrRecordNotFound should be treated
// as an irrecoverable error and indicates a bug.
func (r *RecordCache) Update(originId flow.Identifier) (int64, error) {
	r.Init(originId)
	adjustedEntity, adjusted := r.c.Adjust(originId, func(entity flow.Entity) flow.Entity {
		record, ok := entity.(RecordEntity)
		if !ok {
			// sanity check
			// This should never happen, because the cache only contains RecordEntity entities.
			panic(fmt.Sprintf("invalid entity type, expected RecordEntity type, got: %T", entity))
		}
		record.Counter.Inc()
		// Return the adjusted record.
		return record
	})

	if !adjusted {
		return 0, ErrRecordNotFound
	}

	return adjustedEntity.(RecordEntity).Counter.Load(), nil
}

// Get returns the current number of cluster prefixed topcis received from a peer.
// The record is initialized before the count is returned.
// Before the count is returned it is decayed using the configured decay function.
// Returns the record and true if the record exists, nil and false otherwise.
// Args:
// - originId: the origin id the sender of the control message.
// Returns:
// - The number of cluster prefix topics received after the decay and true if the record exists, 0 and false otherwise.
func (r *RecordCache) Get(originId flow.Identifier) (int64, bool) {
	if r.Init(originId) {
		return 0, true
	}

	entity, ok := r.c.ByID(originId)
	if !ok {
		// sanity check
		// This should never happen because the record should have been initialized in the step at line 114, we should
		// expect the record to always exists before reaching this code.
		panic(fmt.Sprintf("failed to get entity after initialization returned false for entity id %s", originId))
	}

	record, ok := entity.(RecordEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains RecordEntity entities.
		panic(fmt.Sprintf("invalid entity type, expected RecordEntity type, got: %T", entity))
	}

	// perform decay on Counter
	return record.Counter.Load(), true
}

// Identities returns the list of identities of the nodes that have a spam record in the cache.
func (r *RecordCache) Identities() []flow.Identifier {
	return flow.GetIDs(r.c.All())
}

// Remove removes the record of the given peer id from the cache.
// Returns true if the record is removed, false otherwise (i.e., the record does not exist).
// Args:
// - originId: the origin id the sender of the control message.
// Returns:
// - true if the record is removed, false otherwise (i.e., the record does not exist).
func (r *RecordCache) Remove(originId flow.Identifier) bool {
	return r.c.Remove(originId)
}

// Size returns the number of records in the cache.
func (r *RecordCache) Size() uint {
	return r.c.Size()
}

// entityID converts peer ID to flow.Identifier.
// HeroCache uses hash of peer.ID as the unique identifier of the record.
func entityID(peerID peer.ID) flow.Identifier {
	return flow.HashToID([]byte(peerID))
}

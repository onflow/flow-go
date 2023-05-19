package cache

import (
	"crypto/rand"
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

var ErrRecordNotFound = fmt.Errorf("record not found")

type recordEntityFactory func(identifier flow.Identifier) RecordEntity

type RecordCacheConfig struct {
	sizeLimit uint32
	logger    zerolog.Logger
	collector module.HeroCacheMetrics
	// recordDecay decay factor used by the cache to perform geometric decay on counters.
	recordDecay float64
}

// RecordCache is a cache that stores *ClusterPrefixTopicsReceivedRecord used by the control message validation inspector
// to keep track of the amount of cluster prefixed control messages received by a peer.
type RecordCache struct {
	// recordEntityFactory is a factory function that creates a new *RecordEntity.
	recordEntityFactory recordEntityFactory
	// c is the underlying cache.
	c *stdmap.Backend
	// decayFunc decay func used by the cache to perform decay on counters.
	decayFunc preProcessingFunc
	// activeClusterIdsCacheId identifier used to store the active cluster Ids.
	activeClusterIdsCacheId flow.Identifier
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
func NewRecordCache(config *RecordCacheConfig, recordEntityFactory recordEntityFactory) (*RecordCache, error) {
	backData := herocache.NewCache(config.sizeLimit,
		herocache.DefaultOversizeFactor,
		// this cache is supposed to keep the cluster prefix topics received record for the authorized (staked) nodes. Since the number of such nodes is
		// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
		// the records of the authorized nodes. Also, this cache is keeping at most one record per peer id, so the
		// size of the cache must be at least the number of authorized nodes.
		heropool.NoEjection,
		config.logger.With().Str("mempool", "gossipsub=cluster-prefix-topics-received-records").Logger(),
		config.collector)
	recordCache := &RecordCache{
		recordEntityFactory: recordEntityFactory,
		decayFunc:           defaultDecayFunction(config.recordDecay),
		c:                   stdmap.NewBackend(stdmap.WithBackData(backData)),
	}

	var err error
	recordCache.activeClusterIdsCacheId, err = activeClusterIdsKey()
	if err != nil {
		return nil, err
	}
	recordCache.initActiveClusterIds()
	return recordCache, nil
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
// Returns:
//   - The number of cluster prefix topics received after the adjustment.
//   - error if the adjustFunc returns an error or if the record does not exist (ErrRecordNotFound).
//     All errors should be treated as an irrecoverable error and indicates a bug.
//
// Note if Adjust is called under the assumption that the record exists, the ErrRecordNotFound should be treated
// as an irrecoverable error and indicates a bug.
func (r *RecordCache) Update(originId flow.Identifier) (float64, error) {
	optimisticAdjustFunc := func() (flow.Entity, bool) {
		return r.c.Adjust(originId, func(entity flow.Entity) flow.Entity {
			r.decayAdjustment(entity)            // first decay the record
			return r.incrementAdjustment(entity) // then increment the record
		})
	}

	// optimisticAdjustFunc is called assuming the record exists; if the record does not exist,
	// it means the record was not initialized. In this case, initialize the record and call optimisticAdjustFunc again.
	// If the record was initialized, optimisticAdjustFunc will be called only once.
	adjustedEntity, ok := optimisticAdjustFunc()
	if !ok {
		r.Init(originId)
		adjustedEntity, ok = optimisticAdjustFunc()
		if !ok {
			return 0, fmt.Errorf("record not found for origin id %s, even after an init attempt", originId)
		}
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
func (r *RecordCache) Get(originId flow.Identifier) (float64, bool, error) {
	if r.Init(originId) {
		return 0, true, nil
	}

	adjustedEntity, adjusted := r.c.Adjust(originId, r.decayAdjustment)
	if !adjusted {
		return 0, false, ErrRecordNotFound
	}

	record, ok := adjustedEntity.(RecordEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains RecordEntity entities.
		panic(fmt.Sprintf("invalid entity type, expected RecordEntity type, got: %T", adjustedEntity))
	}

	// perform decay on Counter
	return record.Counter.Load(), true, nil
}

func (r *RecordCache) storeActiveClusterIds(clusterIDList flow.ChainIDList) flow.ChainIDList {
	adjustedEntity, _ := r.c.Adjust(r.activeClusterIdsCacheId, func(entity flow.Entity) flow.Entity {
		record, ok := entity.(ActiveClusterIdsEntity)
		if !ok {
			// sanity check
			// This should never happen, because cache should always contain a ActiveClusterIdsEntity
			// stored at the flow.ZeroID
			panic(fmt.Sprintf("invalid entity type, expected ActiveClusterIdsEntity type, got: %T", entity))
		}
		record.ActiveClusterIds = clusterIDList
		// Return the adjusted record.
		return record
	})
	return adjustedEntity.(ActiveClusterIdsEntity).ActiveClusterIds
}

func (r *RecordCache) getActiveClusterIds() flow.ChainIDList {
	adjustedEntity, ok := r.c.ByID(r.activeClusterIdsCacheId)
	if !ok {
		// sanity check
		// This should never happen, because cache should always contain a ActiveClusterIdsEntity
		// stored at the flow.ZeroID
		panic(fmt.Sprintf("invalid entity type, expected ActiveClusterIdsEntity type, got: %T", adjustedEntity))
	}
	return adjustedEntity.(ActiveClusterIdsEntity).ActiveClusterIds
}

func (r *RecordCache) initActiveClusterIds() {
	activeClusterIdsEntity := NewActiveClusterIdsEntity(r.activeClusterIdsCacheId, make(flow.ChainIDList, 0))
	stored := r.c.Add(activeClusterIdsEntity)
	if !stored {
		panic("failed to initialize active cluster Ids in RecordCache")
	}
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

func (r *RecordCache) incrementAdjustment(entity flow.Entity) flow.Entity {
	record, ok := entity.(RecordEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains RecordEntity entities.
		panic(fmt.Sprintf("invalid entity type, expected RecordEntity type, got: %T", entity))
	}
	record.Counter.Add(1)
	record.lastUpdated = time.Now()
	// Return the adjusted record.
	return record
}

func (r *RecordCache) decayAdjustment(entity flow.Entity) flow.Entity {
	record, ok := entity.(RecordEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains RecordEntity entities.
		panic(fmt.Sprintf("invalid entity type, expected RecordEntity type, got: %T", entity))
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

func (r *RecordCache) getActiveClusterIdsCacheId() flow.Identifier {
	return r.activeClusterIdsCacheId
}

type preProcessingFunc func(recordEntity RecordEntity) (RecordEntity, error)

// defaultDecayFunction is the default decay function that is used to decay the cluster prefixed topic received counter of a peer.
func defaultDecayFunction(decay float64) preProcessingFunc {
	return func(recordEntity RecordEntity) (RecordEntity, error) {
		if recordEntity.Counter.Load() == 0 {
			return recordEntity, nil
		}

		decayedVal, err := scoring.GeometricDecay(recordEntity.Counter.Load(), decay, recordEntity.lastUpdated)
		if err != nil {
			return recordEntity, fmt.Errorf("could not decay cluster prefixed topic received counter: %w", err)
		}
		recordEntity.Counter.Store(decayedVal)
		return recordEntity, nil
	}
}

// activeClusterIdsKey returns the key used to store the active cluster ids in the cache.
// The key is a random string that is generated once and stored in the cache.
// The key is used to retrieve the active cluster ids from the cache.
// Args:
// none
// Returns:
// - the key used to store the active cluster ids in the cache.
// - an error if the key could not be generated (irrecoverable).
func activeClusterIdsKey() (flow.Identifier, error) {
	salt := make([]byte, 100)
	_, err := rand.Read(salt)
	if err != nil {
		return flow.Identifier{}, err
	}
	return flow.MakeID(fmt.Sprintf("active-cluster-ids-%x", salt)), nil
}

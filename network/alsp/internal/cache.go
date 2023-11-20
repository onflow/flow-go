package internal

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/network/alsp/model"
)

var ErrSpamRecordNotFound = fmt.Errorf("spam record not found")

// SpamRecordCache is a cache that stores spam records at the protocol layer for ALSP.
type SpamRecordCache struct {
	recordFactory model.SpamRecordFactoryFunc // recordFactory is a factory function that creates a new spam record.
	c             *stdmap.Backend             // c is the underlying cache.
}

var _ alsp.SpamRecordCache = (*SpamRecordCache)(nil)

// NewSpamRecordCache creates a new SpamRecordCache.
// Args:
// - sizeLimit: the maximum number of records that the cache can hold.
// - logger: the logger used by the cache.
// - collector: the metrics collector used by the cache.
// - recordFactory: a factory function that creates a new spam record.
// Returns:
// - *SpamRecordCache, the created cache.
// Note that the cache is supposed to keep the spam record for the authorized (staked) nodes. Since the number of such nodes is
// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
// the spam records of the authorized nodes. Also, this cache is keeping at most one record per origin id, so the
// size of the cache must be at least the number of authorized nodes.
func NewSpamRecordCache(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics, recordFactory model.SpamRecordFactoryFunc) *SpamRecordCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		// this cache is supposed to keep the spam record for the authorized (staked) nodes. Since the number of such nodes is
		// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
		// the spam records of the authorized nodes. Also, this cache is keeping at most one record per origin id, so the
		// size of the cache must be at least the number of authorized nodes.
		heropool.NoEjection,
		logger.With().Str("mempool", "aslp-spam-records").Logger(),
		collector)

	return &SpamRecordCache{
		recordFactory: recordFactory,
		c:             stdmap.NewBackend(stdmap.WithBackData(backData)),
	}
}

// init initializes the spam record cache for the given origin id if it does not exist.
// Returns true if the record is initialized, false otherwise (i.e., the record already exists).
// Args:
// - originId: the origin id of the spam record.
// Returns:
// - true if the record is initialized, false otherwise (i.e., the record already exists).
// Note that if Init is called multiple times for the same origin id, the record is initialized only once, and the
// subsequent calls return false and do not change the record (i.e., the record is not re-initialized).
func (s *SpamRecordCache) init(originId flow.Identifier) bool {
	return s.c.Add(ProtocolSpamRecordEntity{s.recordFactory(originId)})
}

// Adjust applies the given adjust function to the spam record of the given origin id.
// Returns the Penalty value of the record after the adjustment.
// It returns an error if the adjustFunc returns an error or if the record does not exist.
// Note that if the record the Adjust is called when the record does not exist, the record is initialized and the
// adjust function is applied to the initialized record again. In this case, the adjust function should not return an error.
// Args:
// - originId: the origin id of the spam record.
// - adjustFunc: the function that adjusts the spam record.
// Returns:
//   - Penalty value of the record after the adjustment.
//   - error any returned error should be considered as an irrecoverable error and indicates a bug.
func (s *SpamRecordCache) Adjust(originId flow.Identifier, adjustFunc model.RecordAdjustFunc) (float64, error) {
	// first, we try to optimistically adjust the record assuming that the record already exists.
	penalty, err := s.adjust(originId, adjustFunc)

	switch {
	case err == ErrSpamRecordNotFound:
		// if the record does not exist, we initialize the record and try to adjust it again.
		// Note: there is an edge case where the record is initialized by another goroutine between the two calls.
		// In this case, the init function is invoked twice, but it is not a problem because the underlying
		// cache is thread-safe. Hence, we do not need to synchronize the two calls. In such cases, one of the
		// two calls returns false, and the other call returns true. We do not care which call returns false, hence,
		// we ignore the return value of the init function.
		_ = s.init(originId)
		// as the record is initialized, the adjust function should not return an error, and any returned error
		// is an irrecoverable error and indicates a bug.
		return s.adjust(originId, adjustFunc)
	case err != nil:
		// if the adjust function returns an unexpected error on the first attempt, we return the error directly.
		return 0, err
	default:
		// if the adjust function returns no error, we return the penalty value.
		return penalty, nil
	}
}

// adjust applies the given adjust function to the spam record of the given origin id.
// Returns the Penalty value of the record after the adjustment.
// It returns an error if the adjustFunc returns an error or if the record does not exist.
// Args:
// - originId: the origin id of the spam record.
// - adjustFunc: the function that adjusts the spam record.
// Returns:
//   - Penalty value of the record after the adjustment.
//   - error if the adjustFunc returns an error or if the record does not exist (ErrSpamRecordNotFound). Except the ErrSpamRecordNotFound,
//     any other error should be treated as an irrecoverable error and indicates a bug.
func (s *SpamRecordCache) adjust(originId flow.Identifier, adjustFunc model.RecordAdjustFunc) (float64, error) {
	var rErr error
	adjustedEntity, adjusted := s.c.Adjust(originId, func(entity flow.Entity) flow.Entity {
		record, ok := entity.(ProtocolSpamRecordEntity)
		if !ok {
			// sanity check
			// This should never happen, because the cache only contains ProtocolSpamRecordEntity entities.
			panic(fmt.Sprintf("invalid entity type, expected ProtocolSpamRecordEntity type, got: %T", entity))
		}

		// Adjust the record.
		adjustedRecord, err := adjustFunc(record.ProtocolSpamRecord)
		if err != nil {
			rErr = fmt.Errorf("adjust function failed: %w", err)
			return entity // returns the original entity (reverse the adjustment).
		}

		// Return the adjusted record.
		return ProtocolSpamRecordEntity{adjustedRecord}
	})

	if rErr != nil {
		return 0, fmt.Errorf("failed to adjust record: %w", rErr)
	}

	if !adjusted {
		return 0, ErrSpamRecordNotFound
	}

	return adjustedEntity.(ProtocolSpamRecordEntity).Penalty, nil
}

// Get returns the spam record of the given origin id.
// Returns the record and true if the record exists, nil and false otherwise.
// Args:
// - originId: the origin id of the spam record.
// Returns:
// - the record and true if the record exists, nil and false otherwise.
// Note that the returned record is a copy of the record in the cache (we do not want the caller to modify the record).
func (s *SpamRecordCache) Get(originId flow.Identifier) (*model.ProtocolSpamRecord, bool) {
	entity, ok := s.c.ByID(originId)
	if !ok {
		return nil, false
	}

	record, ok := entity.(ProtocolSpamRecordEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains ProtocolSpamRecordEntity entities.
		panic(fmt.Sprintf("invalid entity type, expected ProtocolSpamRecordEntity type, got: %T", entity))
	}

	// return a copy of the record (we do not want the caller to modify the record).
	return &model.ProtocolSpamRecord{
		OriginId:       record.OriginId,
		Decay:          record.Decay,
		CutoffCounter:  record.CutoffCounter,
		Penalty:        record.Penalty,
		DisallowListed: record.DisallowListed,
	}, true
}

// Identities returns the list of identities of the nodes that have a spam record in the cache.
func (s *SpamRecordCache) Identities() []flow.Identifier {
	return flow.GetIDs(s.c.All())
}

// Remove removes the spam record of the given origin id from the cache.
// Returns true if the record is removed, false otherwise (i.e., the record does not exist).
// Args:
// - originId: the origin id of the spam record.
// Returns:
// - true if the record is removed, false otherwise (i.e., the record does not exist).
func (s *SpamRecordCache) Remove(originId flow.Identifier) bool {
	return s.c.Remove(originId)
}

// Size returns the number of spam records in the cache.
func (s *SpamRecordCache) Size() uint {
	return s.c.Size()
}

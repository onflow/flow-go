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
)

// SpamRecordCache is a cache that stores spam records at the protocol layer for ALSP.
type SpamRecordCache struct {
	recordFactory func(flow.Identifier) alsp.ProtocolSpamRecord // recordFactory is a factory function that creates a new spam record.
	c             *stdmap.Backend                               // c is the underlying cache.
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
func NewSpamRecordCache(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics, recordFactory func(flow.Identifier) alsp.ProtocolSpamRecord) *SpamRecordCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		// this cache is supposed to keep the spam record for the authorized (staked) nodes. Since the number of such nodes is
		// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
		// the spam records of the authorized nodes. Also, this cache is keeping at most one record per origin id, so the
		// size of the cache must be at least the number of authorized nodes.
		heropool.NoEjection,
		logger.With().Str("mempool", "aslp=spam-records").Logger(),
		collector)

	return &SpamRecordCache{
		recordFactory: recordFactory,
		c:             stdmap.NewBackend(stdmap.WithBackData(backData)),
	}
}

// Init initializes the spam record cache for the given origin id if it does not exist.
// Returns true if the record is initialized, false otherwise (i.e., the record already exists).
// Args:
// - originId: the origin id of the spam record.
// Returns:
// - true if the record is initialized, false otherwise (i.e., the record already exists).
// Note that if Init is called multiple times for the same origin id, the record is initialized only once, and the
// subsequent calls return false and do not change the record (i.e., the record is not re-initialized).
func (s *SpamRecordCache) Init(originId flow.Identifier) bool {
	return s.c.Add(ProtocolSpamRecordEntity{s.recordFactory(originId)})
}

// Adjust applies the given adjust function to the spam record of the given origin id.
// Returns the Penalty value of the record after the adjustment.
// It returns an error if the adjustFunc returns an error or if the record does not exist.
// Assuming that adjust is always called when the record exists, the error is irrecoverable and indicates a bug.
// Args:
// - originId: the origin id of the spam record.
// - adjustFunc: the function that adjusts the spam record.
// Returns:
// - Penalty value of the record after the adjustment.
func (s *SpamRecordCache) Adjust(originId flow.Identifier, adjustFunc alsp.RecordAdjustFunc) (float64, error) {
	var rErr error
	adjustedEntity, adjusted := s.c.Adjust(originId, func(entity flow.Entity) flow.Entity {
		record, ok := entity.(ProtocolSpamRecordEntity)
		if !ok {
			// sanity check
			// This should never happen, because the cache only contains ProtocolSpamRecordEntity entities.
			panic("invalid entity type, expected ProtocolSpamRecordEntity type")
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
		return 0, fmt.Errorf("record does not exist")
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
func (s *SpamRecordCache) Get(originId flow.Identifier) (*alsp.ProtocolSpamRecord, bool) {
	entity, ok := s.c.ByID(originId)
	if !ok {
		return nil, false
	}

	record, ok := entity.(ProtocolSpamRecordEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains ProtocolSpamRecordEntity entities.
		panic("invalid entity type, expected ProtocolSpamRecordEntity type")
	}

	// return a copy of the record (we do not want the caller to modify the record).
	return &alsp.ProtocolSpamRecord{
		OriginId:      record.OriginId,
		Decay:         record.Decay,
		CutoffCounter: record.CutoffCounter,
		Penalty:       record.Penalty,
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

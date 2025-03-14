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

// SpamRecordCache is a cache that stores spam records at the protocol layer for ALSP.
type SpamRecordCache struct {
	recordFactory model.SpamRecordFactoryFunc                                 // recordFactory is a factory function that creates a new spam record.
	c             *stdmap.Backend[flow.Identifier, *model.ProtocolSpamRecord] // c is the underlying cache.
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
	backData := herocache.NewCache[*model.ProtocolSpamRecord](sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		logger.With().Str("mempool", "aslp-spam-records").Logger(),
		collector)

	return &SpamRecordCache{
		recordFactory: recordFactory,
		c:             stdmap.NewBackend(stdmap.WithMutableBackData[flow.Identifier, *model.ProtocolSpamRecord](backData)),
	}
}

// AdjustWithInit applies the given adjust function to the spam record of the given origin id.
// Returns the Penalty value of the record after the adjustment.
// It returns an error if the adjustFunc returns an error or if the record does not exist.
// Note that if the record does not exist, the record is initialized and the
// adjust function is applied to the initialized record again.
// Args:
// - originId: the origin id of the spam record.
// - adjustFunc: the function that adjusts the spam record.
// Returns:
//   - Penalty value of the record after the adjustment.
//   - error any returned error should be considered as an irrecoverable error and indicates a bug.
func (s *SpamRecordCache) AdjustWithInit(originId flow.Identifier, adjustFunc model.RecordAdjustFunc) (float64, error) {
	var rErr error
	wrapAdjustFunc := func(record *model.ProtocolSpamRecord) *model.ProtocolSpamRecord {
		// Adjust the record.
		adjustedRecord, err := adjustFunc(record)
		if err != nil {
			rErr = fmt.Errorf("adjust function failed: %w", err)
			return record // returns the original record (reverse the adjustment).
		}

		// Return the adjusted record.
		return adjustedRecord
	}
	initFunc := func() *model.ProtocolSpamRecord {
		return s.recordFactory(originId)
	}

	adjustedRecord, adjusted := s.c.AdjustWithInit(originId, wrapAdjustFunc, initFunc)
	if rErr != nil {
		return 0, fmt.Errorf("failed to adjust record: %w", rErr)
	}

	if !adjusted {
		return 0, fmt.Errorf("adjustment failed for origin id %s", originId)
	}

	return adjustedRecord.Penalty, nil
}

// Get returns the spam record of the given origin id.
// Returns the record and true if the record exists, nil and false otherwise.
// Args:
// - originId: the origin id of the spam record.
// Returns:
// - the record and true if the record exists, nil and false otherwise.
// Note that the returned record is a copy of the record in the cache (we do not want the caller to modify the record).
func (s *SpamRecordCache) Get(originId flow.Identifier) (*model.ProtocolSpamRecord, bool) {
	return s.c.Get(originId)
}

// Identities returns the list of identities of the nodes that have a spam record in the cache.
func (s *SpamRecordCache) Identities() []flow.Identifier {
	all := s.c.All()
	identifiers := make(flow.IdentifierList, 0, len(all))
	for identifier, _ := range all {
		identifiers = append(identifiers, identifier)
	}
	return identifiers
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

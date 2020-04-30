package cache

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type PayloadCache struct {
	lookback     uint64
	guaranteeIDs map[uint64][]flow.Identifier
	sealIDs      map[uint64][]flow.Identifier
}

func NewPayloadCache(db *badger.DB) (*PayloadCache, error) {

	pc := &PayloadCache{
		lookback:     10000,
		guaranteeIDs: make(map[uint64][]flow.Identifier),
		sealIDs:      make(map[uint64][]flow.Identifier),
	}

	err := db.View(func(tx *badger.Txn) error {

		// get finalized boundary
		var boundary uint64
		err := operation.RetrieveBoundary(&boundary)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// get the finalized block ID
		var finalID flow.Identifier
		err = operation.RetrieveNumber(boundary, &finalID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve final ID: %w", err)
		}

		// calculate the limit
		limit := boundary - pc.lookback
		if limit > pc.lookback { // overflow check
			limit = 0
		}

		// go from last finalized to limit block to load payload IDs
		err = operation.LookupGuaranteeHistory(boundary, limit, finalID, &pc.guaranteeIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not look up guarantee history: %w", err)
		}
		err = operation.LookupSealHistory(boundary, limit, finalID, &pc.sealIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not look up seal history: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap payload cache: %w", err)
	}

	return pc, nil
}

func (pc *PayloadCache) AddGuarantees(height uint64, guaranteeIDs []flow.Identifier) {

	// first, insert the new IDs
	// TODO: maybe we should double check if we already have it?
	pc.guaranteeIDs[height] = guaranteeIDs

	// delete all heights that are too far back now
	cutoff := height - pc.lookback
	if cutoff > height { // if we overflow, nothing to remove
		return
	}
	for old := range pc.guaranteeIDs {
		if old < cutoff {
			delete(pc.guaranteeIDs, old)
		}
	}
}

func (pc *PayloadCache) AddSeals(height uint64, sealIDs []flow.Identifier) {

	// first, insert the new IDs
	// TODO: maybe we should double check if we already have it?
	pc.sealIDs[height] = sealIDs

	// delete all heights that are too far back now
	cutoff := height - pc.lookback
	if cutoff > height { // if we overflow, nothing to remove
		return
	}
	for old := range pc.sealIDs {
		if old < cutoff {
			delete(pc.sealIDs, old)
		}
	}
}

func (pc *PayloadCache) HasGuarantee(guaranteeID flow.Identifier) bool {
	for _, checkIDs := range pc.guaranteeIDs {
		for _, checkID := range checkIDs {
			if checkID == guaranteeID {
				return true
			}
		}
	}
	return false
}

func (pc *PayloadCache) HasSeal(sealID flow.Identifier) bool {
	for _, checkIDs := range pc.sealIDs {
		for _, checkID := range checkIDs {
			if checkID == sealID {
				return true
			}
		}
	}
	return false
}

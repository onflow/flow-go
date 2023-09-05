package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// RegisterIndex defines methods for the register index.
type RegisterIndex interface {
	RegisterIndexReader
	RegisterIndexWriter
}

// RegisterIndexReader defines read-only operations on the register index.
type RegisterIndexReader interface {
	// LatestHeight at which we indexed registers.
	LatestHeight() (uint64, error)
	// FirstHeight at which we started to index the registers.
	// Returns the first indexed height found in the store.
	FirstHeight() (uint64, error)
	// Get register by the register ID at a given block height.
	//
	// If the register at the given height was not indexed, returns the highest
	// height the register was indexed at.
	// An error is returned if the register was not indexed at all or if the height is out of bounds.
	// Expected errors:
	// - storage.ErrNotFound if the register was not found in the db or is out of bounds.
	Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

// RegisterIndexWriter defines write-only operations on the register index.
type RegisterIndexWriter interface {
	// Store batch of register entries at the provided block height.
	// The provided height should either be one higher than the current height or the same to ensure idempotency.
	// If the height is not within those bounds it will panic!
	// Store should be used with the SetLatestHeight to progress the indexing.
	// An error might get returned if there are problems with persisting the registers.
	Store(entries flow.RegisterEntries, height uint64) error
	// SetLatestHeight updates the latest height record.
	// The provided height should either be one higher than the current height or the same to ensure idempotency.
	// If the height is not within those bounds it will panic!
	// An error might get returned if there are problems with persisting the height.
	SetLatestHeight(height uint64) error
}

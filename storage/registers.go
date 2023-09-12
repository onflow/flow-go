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
	// Get register by the register ID at a given block height.
	//
	// If the register at the given height was not indexed, returns the highest
	// height the register was indexed at.
	// An error is returned if the register was not indexed at all or if the height is out of bounds.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the register was not found in the db or is out of bounds.
	Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)
	// LatestHeight returns the latest indexed height.
	LatestHeight() (uint64, error)
	// FirstHeight at which we started to index. Returns the first indexed height found in the store.
	FirstHeight() (uint64, error)
}

// RegisterIndexWriter defines write-only operations on the register index.
type RegisterIndexWriter interface {
	// Store batch of register entries at the provided block height.
	//
	// The provided height must either be one higher than the current height or the same to ensure idempotency,
	// otherwise and error is returned. If the height is not within those bounds there is either a bug
	// or state corruption.
	//
	// No errors are expected during normal operation.
	Store(entries flow.RegisterEntries, height uint64) error
}

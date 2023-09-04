package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Registers defines methods for the register index.
type Registers interface {
	RegisterReader
	RegisterWriter
}

// RegisterReader defines read-only operations on the register index.
type RegisterReader interface {
	// LatestHeight at which we indexed registers.
	LatestHeight() (uint64, error)
	// Get register by the register ID at a given block height.
	//
	// If the register at the given height was not indexed, returns the highest
	// height the register was indexed at.
	// An error is returned if the register was not indexed at all or if the height is out of bounds.
	// Expected errors:
	// - storage.ErrNotFound if the register was not found in the db or is out of bounds.
	Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

// RegisterWriter defines write-only operations on the register index.
type RegisterWriter interface {
	// Store batch of register entries at the provided block height.
	// The provided height should either be one higher than the current height or the same to ensure idempotency.
	// If the height is not within those bounds it will panic!
	Store(entries flow.RegisterEntries, height uint64) error
}

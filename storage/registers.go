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
	// Get register by the register ID at a given block height.
	//
	// If the register at the given height was not indexed, returns the highest height the register was indexed at.
	// An error is returned if the register was not indexed at all. Expected errors:
	// - storage.ErrNotFound if the register was not found in the db
	Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

// RegisterWriter defines write-only operations on the register index.
type RegisterWriter interface {
	// Store batch of register entries at the provided block height.
	//
	// If the registers already exists it overwrites them to make this action idempotent.
	Store(entries flow.RegisterEntries, height uint64) error
}

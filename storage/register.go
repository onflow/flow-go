package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"io"
)

// Register defines methods for the register index.
type Register interface {
	RegisterReader
	RegisterWriter
	io.Closer
}

// RegisterReader defines read-only operations on the register index.
type RegisterReader interface {
	// Get register by the register ID at a given block height.
	//
	// If the register at the given height was not indexed, returns the highest
	// height the register was indexed at.
	// An error is returned if the register was not indexed at all.
	Get(ID flow.RegisterID, height uint64) (flow.RegisterEntry, error)
}

// RegisterWriter defines write-only operations on the register index.
type RegisterWriter interface {
	// Store batch of register entries at the provided block height.
	Store(entries flow.RegisterEntries, height uint64) error
}

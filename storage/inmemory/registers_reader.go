package inmemory

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RegistersReader is a simple in-memory implementation of the RegisterIndexReader interface.
// It stores registers for a single block height.
type RegistersReader struct {
	blockHeight uint64
	store       map[flow.RegisterID]flow.RegisterEntry
}

var _ storage.RegisterIndexReader = (*RegistersReader)(nil)

func NewRegisters(blockHeight uint64, registers []flow.RegisterEntry) *RegistersReader {
	store := make(map[flow.RegisterID]flow.RegisterEntry)
	for _, reg := range registers {
		store[reg.Key] = reg
	}
	return &RegistersReader{
		blockHeight: blockHeight,
		store:       store,
	}
}

// Get returns a register by the register ID at a storage's block height.
//
// Expected error returns during normal operation:
// - [storage.ErrNotFound] if the register does not exist in this storage object
// - [storage.ErrHeightNotIndexed] if the given height does not match the storage's block height
func (r *RegistersReader) Get(registerID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	if r.blockHeight != height {
		return flow.RegisterValue{}, storage.ErrHeightNotIndexed
	}

	reg, ok := r.store[registerID]
	if !ok {
		return flow.RegisterValue{}, storage.ErrNotFound
	}

	return reg.Value, nil
}

// LatestHeight returns the latest indexed height.
func (r *RegistersReader) LatestHeight() uint64 {
	return r.blockHeight
}

// FirstHeight returns the first indexed height found in the store.
func (r *RegistersReader) FirstHeight() uint64 {
	return r.blockHeight
}

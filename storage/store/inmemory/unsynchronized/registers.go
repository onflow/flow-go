package unsynchronized

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Registers is a simple in-memory implementation of the RegisterIndex interface.
// It stores registers for a single block height.
type Registers struct {
	blockHeight uint64
	lock        sync.RWMutex
	store       map[flow.RegisterID]flow.RegisterValue
}

var _ storage.RegisterIndex = (*Registers)(nil)

func NewRegisters(blockHeight uint64) *Registers {
	return &Registers{
		blockHeight: blockHeight,
		store:       make(map[flow.RegisterID]flow.RegisterValue),
	}
}

// Get returns a register by the register ID at a storage's block height.
//
// Expected errors:
// - storage.ErrNotFound if the register does not exist in this storage object
// - storage.ErrHeightNotIndexed if the given height does not match the storage's block height
func (r *Registers) Get(registerID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if r.blockHeight != height {
		return flow.RegisterValue{}, storage.ErrHeightNotIndexed
	}

	reg, ok := r.store[registerID]
	if !ok {
		return flow.RegisterValue{}, storage.ErrNotFound
	}

	return reg, nil
}

// LatestHeight returns the latest indexed height.
func (r *Registers) LatestHeight() uint64 {
	return r.blockHeight
}

// FirstHeight returns the first indexed height found in the store.
func (r *Registers) FirstHeight() uint64 {
	return r.blockHeight
}

// Store stores a batch of register entries at the storage's block height.
// No errors are expected during normal operation.
func (r *Registers) Store(registers flow.RegisterEntries, height uint64) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.blockHeight != height {
		return fmt.Errorf("failed to store registers: height mismatch: expected %d, got %d", r.blockHeight, height)
	}

	for _, reg := range registers {
		r.store[reg.Key] = reg.Value
	}

	return nil
}

// Data returns all register entries for the specified height.
// No errors are expected during normal operation.
func (r *Registers) Data(height uint64) ([]flow.RegisterEntry, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if r.blockHeight != height {
		return nil, fmt.Errorf("failed to get registers: height mismatch: expected %d, got %d", r.blockHeight, height)
	}

	out := make([]flow.RegisterEntry, 0, len(r.store))
	for k, v := range r.store {
		out = append(out, flow.RegisterEntry{Key: k, Value: v})
	}
	return out, nil
}

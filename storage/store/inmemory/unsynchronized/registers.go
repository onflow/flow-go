package unsynchronized

import (
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
// The provided argument 'height' is ignored.
//
// Expected errors:
// - storage.ErrNotFound if the register does not exist.
func (r *Registers) Get(registerID flow.RegisterID, _ uint64) (flow.RegisterValue, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if reg, ok := r.store[registerID]; ok {
		return reg, nil
	}

	return flow.RegisterValue{}, storage.ErrNotFound
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
// The provided argument 'height' is ignored.
//
// No errors are expected during normal operation.
func (r *Registers) Store(registers flow.RegisterEntries, _ uint64) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	for _, reg := range registers {
		r.store[reg.Key] = reg.Value
	}

	return nil
}

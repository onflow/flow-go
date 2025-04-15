package unsynchronized

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type RegisterEntries map[flow.RegisterID]flow.RegisterValue
type HeightToRegisterEntries map[uint64]RegisterEntries

type Registers struct {
	firstHeight  uint64
	latestHeight uint64
	lock         sync.RWMutex
	store        HeightToRegisterEntries
}

var _ storage.RegisterIndex = (*Registers)(nil)

func NewRegisters(firstHeight uint64, latestHeight uint64) *Registers {
	return &Registers{
		firstHeight:  firstHeight,
		latestHeight: latestHeight,
		store:        make(HeightToRegisterEntries),
	}
}

// Get register by the register ID at a given block height.
//
// If the register at the given height was not indexed, returns the highest
// height the register was indexed at.
// Expected errors:
// - storage.ErrHeightNotIndexed if the given height was not indexed yet or lower than the first indexed height.
// - storage.ErrNotFound if the given height is indexed, but the register does not exist.
func (r *Registers) Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	h, ok := r.store[height]
	if !ok {
		return flow.RegisterValue{}, storage.ErrNotFound
	}

	val, ok := h[ID]
	if !ok {
		return flow.RegisterValue{}, storage.ErrNotFound
	}

	return val, nil
}

// LatestHeight returns the latest indexed height.
func (r *Registers) LatestHeight() uint64 {
	return r.latestHeight
}

// FirstHeight at which we started to index. Returns the first indexed height found in the store.
func (r *Registers) FirstHeight() uint64 {
	return r.firstHeight
}

// Store batch of register entries at the provided block height.
//
// The provided height must either be one higher than the current height or the same to ensure idempotency,
// otherwise and error is returned. If the height is not within those bounds there is either a bug
// or state corruption.
//
// No errors are expected during normal operation.
func (r *Registers) Store(registers flow.RegisterEntries, height uint64) error {
	if height == r.latestHeight {
		return nil
	}

	if height != r.latestHeight+1 {
		return fmt.Errorf("height mismatch: expected %d, got %d", r.latestHeight+1, height)
	}

	newRegisters := make(RegisterEntries)
	for _, reg := range registers {
		newRegisters[reg.Key] = reg.Value
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	r.latestHeight = height
	r.store[height] = newRegisters

	return nil
}

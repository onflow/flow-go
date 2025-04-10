package unsynchronized

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type RegisterEntries map[flow.RegisterID]flow.RegisterValue
type HeightToRegisterEntries map[uint64]RegisterEntries

type Registers struct {
	firstHeight  uint64
	latestHeight atomic.Uint64
	store        HeightToRegisterEntries
}

func NewRegisters(firstHeight uint64) *Registers {
	return &Registers{
		firstHeight:  firstHeight,
		latestHeight: atomic.Uint64{},
		store:        make(HeightToRegisterEntries),
	}
}

var _ storage.RegisterIndex = (*Registers)(nil)

func (r *Registers) Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
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

func (r *Registers) LatestHeight() uint64 {
	return r.latestHeight.Load()
}

func (r *Registers) FirstHeight() uint64 {
	return r.firstHeight
}

func (r *Registers) Store(entries flow.RegisterEntries, height uint64) error {
	if height > r.latestHeight.Load() {
		r.latestHeight.Store(height)
	}

	// Ensure the map for the given height exists
	if _, exists := r.store[height]; !exists {
		r.store[height] = make(RegisterEntries)
	}

	// Store entries
	for _, entry := range entries {
		r.store[height][entry.Key] = entry.Value
	}

	return nil
}

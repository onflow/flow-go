package unsynchronized

import (
	"sync"
	"encoding/binary"
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
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
//
// Expected errors:
// - storage.ErrHeightNotIndexed if the given height does not match the storage's block height.
func (r *Registers) Store(registers flow.RegisterEntries, height uint64) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.blockHeight != height {
		return storage.ErrHeightNotIndexed
	}

	for _, reg := range registers {
		r.store[reg.Key] = reg.Value
	}

	return nil
}

func (r *Registers) AddToBatch(batch storage.ReaderBatchWriter) error {
	for height, entry := range r.store {
		for id, value := range entry {
			encodedHeight := make([]byte, 8)
			binary.BigEndian.PutUint64(encodedHeight, height)
			key := append(encodedHeight, id.Bytes()...)

			err := operation.UpsertByKey(batch.Writer(), key, value)
			if err != nil {
				return fmt.Errorf("could not persist register entry: %w", err)
			}
		}
	}

	return nil
}

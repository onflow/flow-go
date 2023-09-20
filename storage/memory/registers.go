package memory

import (
	"fmt"
	"math"

	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ storage.RegisterIndex = (*Registers)(nil)

type Registers struct {
	registers    map[flow.RegisterID]map[uint64]flow.RegisterValue
	latestHeight uint64
	firstHeight  uint64
}

func NewRegisters(first uint64, last uint64) *Registers {
	return &Registers{
		registers: make(map[flow.RegisterID]map[uint64]flow.RegisterValue),
	}
}

func (r *Registers) LatestHeight() (uint64, error) {
	if r.latestHeight == 0 {
		return 0, storage.ErrNotFound
	}
	return r.latestHeight, nil
}

func (r *Registers) FirstHeight() (uint64, error) {
	if r.firstHeight == 0 {
		return 0, storage.ErrNotFound
	}

	return r.firstHeight, nil
}

func (r *Registers) Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	reg, ok := r.registers[ID]
	if !ok {
		return nil, errors.Wrap(storage.ErrNotFound, fmt.Sprintf("register by ID %s not found", ID.String()))
	}

	// get all the heights indexed for request register, then iterate over
	// to find the highest height compared to requested height and return it.
	indexedHeights := maps.Keys(r.registers[ID])
	slices.Sort(indexedHeights)
	highestHeight := uint64(math.MaxUint64)
	for i := len(indexedHeights) - 1; i >= 0; i-- {
		if indexedHeights[i] <= height {
			highestHeight = indexedHeights[i]
			break
		}
	}

	if highestHeight == math.MaxUint64 {
		return nil, errors.Wrap(storage.ErrNotFound, fmt.Sprintf("register at height %d or lower not found", height))
	}

	return reg[highestHeight], nil
}

func (r *Registers) Store(entries flow.RegisterEntries, height uint64) error {
	for _, e := range entries {
		if _, ok := r.registers[e.Key]; !ok {
			r.registers[e.Key] = make(map[uint64]flow.RegisterValue)
		}

		r.registers[e.Key][height] = e.Value
	}

	r.latestHeight = height

	return nil
}

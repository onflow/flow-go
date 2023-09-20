package memory

import (
	"fmt"
	"math"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ storage.RegisterIndex = (*Registers)(nil)

type Registers struct {
	registers    map[flow.RegisterID]map[uint64]flow.RegisterValue
	latestHeight *atomic.Uint64
	firstHeight  uint64
	logger       zerolog.Logger
}

func NewRegisters(first uint64, last uint64, log zerolog.Logger) *Registers {
	logger := log.With().Str("component", "execution_indexer_storage").Logger()

	return &Registers{
		firstHeight:  first,
		latestHeight: atomic.NewUint64(last),
		registers:    make(map[flow.RegisterID]map[uint64]flow.RegisterValue),
		logger:       logger,
	}
}

func (r *Registers) LatestHeight() (uint64, error) {
	if r.latestHeight.Load() == math.MaxUint64 {
		return 0, storage.ErrNotFound
	}
	return r.latestHeight.Load(), nil
}

func (r *Registers) FirstHeight() (uint64, error) {
	if r.firstHeight == math.MaxUint64 {
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

		r.logger.Debug().Uint64("height", height).
			Str("value", fmt.Sprintf("%x", e.Value)).
			Msgf("stored register with ID: %s", e.Key.String())

		r.registers[e.Key][height] = e.Value
	}

	r.latestHeight.Store(height)

	return nil
}

package memory

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ storage.RegisterIndex = (*Registers)(nil)

type Registers struct {
	registers    map[uint64]map[flow.RegisterID]flow.RegisterValue
	latestHeight uint64
	firstHeight  uint64
}

func NewRegisters() *Registers {
	return &Registers{
		registers: make(map[uint64]map[flow.RegisterID]flow.RegisterValue),
	}
}

func (r *Registers) LatestHeight() (uint64, error) {
	return r.latestHeight, nil
}

func (r *Registers) FirstHeight() (uint64, error) {
	return r.firstHeight, nil
}

func (r Registers) Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	h, ok := r.registers[height]
	if !ok {
		return nil, fmt.Errorf("height %d for registers not indexed", height)
	}

	entry, ok := h[ID]
	if !ok {
		return nil, fmt.Errorf("register by ID %s not found", ID.String())
	}

	return entry, nil
}

func (r *Registers) Store(entries flow.RegisterEntries, height uint64) error {
	if _, ok := r.registers[height]; !ok {
		r.registers[height] = make(map[flow.RegisterID]flow.RegisterValue)
	}
	for _, e := range entries {
		if _, ok := r.registers[height]; !ok {
			r.registers[height] = make(map[flow.RegisterID]flow.RegisterValue)
		}
		r.registers[height][e.Key] = e.Value
	}

	r.latestHeight = height

	return nil
}

package memory

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ storage.Registers = (*Registers)(nil)

type Registers struct {
	registers map[uint64]map[flow.RegisterID]flow.RegisterValue
}

func NewRegisters() *Registers {
	return &Registers{
		registers: make(map[uint64]map[flow.RegisterID]flow.RegisterValue),
	}
}

func (r Registers) Get(ID flow.RegisterID, height uint64) (flow.RegisterEntry, error) {
	h, ok := r.registers[height]
	if !ok {
		return flow.RegisterEntry{}, fmt.Errorf("height %d for registers not indexed", height)
	}

	entry, ok := h[ID]
	if !ok {
		return flow.RegisterEntry{}, fmt.Errorf("register by ID %s not found", ID.String())
	}

	return flow.RegisterEntry{
		Key:   ID,
		Value: entry,
	}, nil
}

func (r Registers) Store(entries flow.RegisterEntries, height uint64) error {
	for _, e := range entries {
		r.registers[height][e.Key] = e.Value
	}

	return nil
}

func (r Registers) Close() error {
	return nil
}

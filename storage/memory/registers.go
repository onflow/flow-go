package memory

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ storage.Registers = Registers{}

type Registers struct {
	registers map[uint64]map[flow.RegisterID]flow.RegisterValue
}

func NewRegisters() *Registers {
	return &Registers{
		registers: make(map[uint64]map[flow.RegisterID]flow.RegisterValue),
	}
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

func (r Registers) Store(entry flow.RegisterEntry, height uint64) error {
	r.registers[height][entry.Key] = entry.Value
	return nil
}

func (r Registers) Close() error {
	return nil
}

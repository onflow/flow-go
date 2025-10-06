package persisters

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RegistersPersister stores [flow.RegisterEntry] values into the registers db
type RegistersPersister struct {
	data      []flow.RegisterEntry
	registers storage.RegisterIndex
	height    uint64
}

func NewRegistersPersister(
	data []flow.RegisterEntry,
	registers storage.RegisterIndex,
	height uint64,
) *RegistersPersister {
	return &RegistersPersister{
		data:      data,
		registers: registers,
		height:    height,
	}
}

// Persist stores the register entries into the registers db
//
// No error returns are expected during normal operations
func (r *RegistersPersister) Persist() error {
	if err := r.registers.Store(r.data, r.height); err != nil {
		return fmt.Errorf("could not persist registers: %w", err)
	}

	return nil
}

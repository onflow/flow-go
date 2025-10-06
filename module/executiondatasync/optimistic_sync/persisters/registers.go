package persisters

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RegistersPersister stores register data for a single execution result into the registers db in a
// single atomic batch operation.
// To ensure the database only contains data certified by the protocol, the registers persister must
// only be called for sealed execution results.
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

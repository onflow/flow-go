package persisters

import (
	"fmt"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

// RegistersPersister handles registers
type RegistersPersister struct {
	inMemoryRegisters  *unsynchronized.Registers
	permanentRegisters storage.RegisterIndex
	height             uint64
}

func NewRegistersPersister(
	inMemoryRegisters *unsynchronized.Registers,
	permanentRegisters storage.RegisterIndex,
	height uint64,
) *RegistersPersister {
	return &RegistersPersister{
		inMemoryRegisters:  inMemoryRegisters,
		permanentRegisters: permanentRegisters,
		height:             height,
	}
}

// Persist persists registers
// No errors are expected during normal operations
func (r *RegistersPersister) Persist() error {
	registerData, err := r.inMemoryRegisters.Data(r.height)
	if err != nil {
		return fmt.Errorf("could not get data from registers: %w", err)
	}

	if err := r.permanentRegisters.Store(registerData, r.height); err != nil {
		return fmt.Errorf("could not persist registers: %w", err)
	}

	return nil
}

package core

import (
	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/language/runtime"
)

// Computer executes blocks and saves results to the world state.
type Computer struct {
	runtime runtime.Runtime
}

// NewComputer returns a new computer connected to the world state.
func NewComputer(runtime runtime.Runtime) *Computer {
	return &Computer{
		runtime: runtime,
	}
}

func (c *Computer) ExecuteTransaction(tx *types.SignedTransaction, readBlockRegister func(string) []byte) (types.Registers, bool) {
	txRegisters := make(types.Registers)

	var readRegister = func(id string) []byte {
		if value, ok := txRegisters[id]; ok {
			return value
		}

		return readBlockRegister(id)
	}

	var writeRegister = func(id string, value []byte) {
		txRegisters[id] = value
	}

	succeeded := c.runtime.ExecuteScript(
		tx.Transaction.Script,
		readRegister,
		writeRegister,
	)

	return txRegisters, succeeded
}

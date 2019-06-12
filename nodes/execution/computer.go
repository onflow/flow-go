package execution

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/runtime"
)

// Computer executes blocks and saves results to the world state.
type Computer interface {
	ExecuteBlock(block *data.Block) (data.Registers, TransactionResults)
}

// TransactionResults stores the result statuses of multiple transactions.
type TransactionResults map[crypto.Hash]bool

type computer struct {
	runtime        runtime.Runtime
	getTransaction func(crypto.Hash) (*data.Transaction, error)
	readRegister   func(string) []byte
}

// NewComputer returns a new computer connected to the world state.
func NewComputer(runtime runtime.Runtime, getTransaction func(crypto.Hash) (*data.Transaction, error), readRegister func(string) []byte) Computer {
	return &computer{
		runtime:        runtime,
		getTransaction: getTransaction,
		readRegister:   readRegister,
	}
}

func (c *computer) ExecuteBlock(block *data.Block) (data.Registers, TransactionResults) {
	registers := make(data.Registers)
	results := make(TransactionResults)

	for _, txHash := range block.TransactionHashes {
		tx, err := c.getTransaction(txHash)

		if err != nil {
			results[tx.Hash()] = false
			continue
		}

		updatedRegisters, succeeded := c.executeTransaction(tx, registers)

		results[tx.Hash()] = succeeded

		if succeeded {
			registers.Update(updatedRegisters)
		}
	}

	return registers, results
}

func (c *computer) executeTransaction(tx *data.Transaction, initialRegisters data.Registers) (data.Registers, bool) {
	registers := make(data.Registers)

	var readRegister = func(id string) []byte {
		if value, ok := registers[id]; ok {
			return value
		}

		if value, ok := initialRegisters[id]; ok {
			return value
		}

		return c.readRegister(id)
	}

	var writeRegister = func(id string, value []byte) {
		registers[id] = value
	}

	succeeded := c.runtime.ExecuteScript(
		tx.Script,
		readRegister,
		writeRegister,
	)

	return registers, succeeded
}

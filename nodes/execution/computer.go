package execution

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/runtime"
)

// Computer executes blocks and saves results to the world state.
type Computer struct {
	runtime        runtime.Runtime
	getTransaction func(crypto.Hash) (*data.Transaction, error)
	readRegister   func(string) []byte
}

// TransactionResults stores the result statuses of multiple transactions.
type TransactionResults map[crypto.Hash]bool

// NewComputer returns a new computer connected to the world state.
func NewComputer(runtime runtime.Runtime, getTransaction func(crypto.Hash) (*data.Transaction, error), readRegister func(string) []byte) *Computer {
	return &Computer{
		runtime:        runtime,
		getTransaction: getTransaction,
		readRegister:   readRegister,
	}
}

// ExecuteBlock executes the transactions in a block and returns the updated register values.
func (c *Computer) ExecuteBlock(block *data.Block) (data.Registers, TransactionResults, error) {
	blockRegisters := make(data.Registers)
	results := make(TransactionResults)

	for _, txHash := range block.TransactionHashes {
		tx, err := c.getTransaction(txHash)
		if err != nil {
			return blockRegisters, results, err
		}

		updatedRegisters, succeeded := c.executeTransaction(tx, blockRegisters)

		results[tx.Hash()] = succeeded

		if succeeded {
			blockRegisters.Update(updatedRegisters)
		}
	}

	return blockRegisters, results, nil
}

func (c *Computer) executeTransaction(tx *data.Transaction, blockRegisters data.Registers) (data.Registers, bool) {
	txRegisters := make(data.Registers)

	var readRegister = func(id string) []byte {
		if value, ok := txRegisters[id]; ok {
			return value
		}

		if value, ok := blockRegisters[id]; ok {
			return value
		}

		return c.readRegister(id)
	}

	var writeRegister = func(id string, value []byte) {
		txRegisters[id] = value
	}

	succeeded := c.runtime.ExecuteScript(
		tx.Script,
		readRegister,
		writeRegister,
	)

	return txRegisters, succeeded
}

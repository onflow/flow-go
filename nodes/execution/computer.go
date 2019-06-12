package execution

import (
	"github.com/dapperlabs/bamboo-emulator/data"
)

type Computer interface {
	ExecuteBlock(block *data.Block) (Registers, TransactionResults)
}

type Runtime interface {
	ExecuteTransaction(tx *data.Transaction, readRegister func(string) []byte, writeRegister func(string, []byte))
}

type TransactionResults map[data.Hash]bool

type computer struct {
	readRegister func(string) []byte
}

func NewComputer(readRegister func(string) []byte) Computer {
	return &computer{
		readRegister: readRegister,
	}
}

func (c *computer) ExecuteBlock(block *data.Block) (Registers, TransactionResults) {
	registers := make(data.Regissters)
	results := make(TransactionResults)

	for _, tx := range block.TransactionHashes() {
		updatedRegisters, succeeded := c.executeTransaction(tx, registers)

		results[tx.Hash()] = succeeded

		if succeeded {
			registers.Update(updatedRegisters)
		}
	}

	return registers, results
}

func (c *computer) executeTransaction(tx *data.Transaction, initialRegisters Registers) (Registers, bool) {
	registers := make(data.Registers)

	func readRegister(id string) []byte {
		value, ok := registers[id]; ok {
			return value
		}

		value, ok = initialRegisters[id]; ok {
			return value
		}

		return c.readRegister(id)
	}

	func writeRegister(id string, value []byte) {
		registers[id] = value
	}

	succeeded := c.runtime.ExecuteTransaction(
		tx,
		readRegister,
		writeRegister,
	)

	return registers, succeeded
}

package execution

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
)

type Computer interface {
	ExecuteBlock(block *data.Block) (data.Registers, TransactionResults)
}

type Runtime interface {
	ExecuteTransaction(tx *data.Transaction, readRegister func(string) []byte, writeRegister func(string, []byte)) bool
}

type TransactionResults map[crypto.Hash]bool

type computer struct {
	getTransaction func(crypto.Hash) (*data.Transaction, error)
	readRegister   func(string) []byte
	runtime        Runtime
}

func NewComputer(getTransaction func(crypto.Hash) (*data.Transaction, error), readRegister func(string) []byte) Computer {
	return &computer{
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

	succeeded := c.runtime.ExecuteTransaction(
		tx,
		readRegister,
		writeRegister,
	)

	return registers, succeeded
}

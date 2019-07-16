package core

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
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

type runtimeInterface struct {
	getValue func(controller, owner, key []byte) (value []byte, err error)
	setValue func(controller, owner, key, value []byte) (err error)
}

func (i *runtimeInterface) GetValue(controller, owner, key []byte) ([]byte, error) {
	return i.getValue(controller, owner, key)
}

func (i *runtimeInterface) SetValue(controller, owner, key, value []byte) error {
	return i.setValue(controller, owner, key, value)
}

func getFullKey(controller, owner, key []byte) crypto.Hash {
	fullKey := append(controller, owner...)
	fullKey = append(fullKey, key...)

	return crypto.NewHash(fullKey)
}

func (c *Computer) ExecuteTransaction(tx *types.SignedTransaction, readBlockRegister func(crypto.Hash) []byte) (types.Registers, bool) {
	txRegisters := make(types.Registers)

	runtimeInterface := &runtimeInterface{
		getValue: func(controller, owner, key []byte) ([]byte, error) {
			fullKey := getFullKey(controller, owner, key)
			return readBlockRegister(fullKey), nil
		},
		setValue: func(controller, owner, key, value []byte) error {
			fullKey := getFullKey(controller, owner, key)
			txRegisters[fullKey] = value
			return nil
		},
	}

	err := c.runtime.ExecuteScript(tx.Transaction.Script, runtimeInterface)

	fmt.Println("REGISTERS", txRegisters)

	return txRegisters, err == nil
}

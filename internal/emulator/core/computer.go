package core

import (
	"github.com/dapperlabs/bamboo-node/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

// Computer provides an interface to execute scripts against the world state.
type Computer struct {
	runtime runtime.Runtime
}

// NewComputer returns a new computer instance.
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

// ExecuteTransaction executes a transaction script against the current world state.
func (c *Computer) ExecuteTransaction(
	tx *types.SignedTransaction,
	readRegister func(crypto.Hash) []byte,
) (etypes.Registers, error) {
	registers := make(etypes.Registers)

	runtimeInterface := &runtimeInterface{
		getValue: func(controller, owner, key []byte) ([]byte, error) {
			fullKey := getFullKey(controller, owner, key)

			if v, ok := registers[fullKey]; ok {
				return v, nil
			}

			return readRegister(fullKey), nil
		},
		setValue: func(controller, owner, key, value []byte) error {
			fullKey := getFullKey(controller, owner, key)
			registers[fullKey] = value
			return nil
		},
	}

	_, err := c.runtime.ExecuteScript(tx.Script, runtimeInterface)

	return registers, err
}

// ExecuteCall executes a read-only script against the current world state.
func (c *Computer) ExecuteCall(
	script []byte,
	readRegister func(crypto.Hash) []byte,
) (interface{}, error) {
	registers := make(etypes.Registers)

	runtimeInterface := &runtimeInterface{
		getValue: func(controller, owner, key []byte) ([]byte, error) {
			fullKey := getFullKey(controller, owner, key)

			if v, ok := registers[fullKey]; ok {
				return v, nil
			}

			return readRegister(fullKey), nil
		},
		setValue: func(controller, owner, key, value []byte) error {
			fullKey := getFullKey(controller, owner, key)
			registers[fullKey] = value
			return nil
		},
	}

	return c.runtime.ExecuteScript(script, runtimeInterface)
}

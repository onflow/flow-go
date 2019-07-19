package core

import (
	"math/big"

	"github.com/dapperlabs/bamboo-node/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	"github.com/dapperlabs/bamboo-node/internal/emulator/state"
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
	getValue      func(controller, owner, key []byte) (value []byte, err error)
	setValue      func(controller, owner, key, value []byte) (err error)
	createAccount func(key, code []byte) (id []byte, err error)
}

func (i *runtimeInterface) GetValue(controller, owner, key []byte) ([]byte, error) {
	return i.getValue(controller, owner, key)
}

func (i *runtimeInterface) SetValue(controller, owner, key, value []byte) error {
	return i.setValue(controller, owner, key, value)
}

func (i *runtimeInterface) CreateAccount(key, code []byte) (id []byte, err error) {
	return i.createAccount(key, code)
}

// ExecuteTransaction executes a transaction against the current world state.
func (c *Computer) ExecuteTransaction(
	tx *types.SignedTransaction,
	stateRegisters state.Registers,
) (registers state.Registers, err error) {
	// TODO: deduct gas cost from transaction signer's account
	_, registers, err = c.ExecuteScript(tx.Script, stateRegisters)
	return registers, err
}

// ExecuteScript executes a script against the current world state.
func (c *Computer) ExecuteScript(
	script []byte,
	stateRegisters state.Registers,
) (result interface{}, registers state.Registers, err error) {
	registers = make(state.Registers)

	runtimeInterface := &runtimeInterface{
		getValue: func(controller, owner, key []byte) ([]byte, error) {
			if v, ok := registers.Get(controller, owner, key); ok {
				return v, nil
			}

			v, _ := stateRegisters.Get(controller, owner, key)
			return v, nil
		},
		setValue: func(controller, owner, key, value []byte) error {
			registers.Set(controller, owner, key, value)
			return nil
		},
		createAccount: func(key, code []byte) (id []byte, err error) {
			latestAccountID, _ := registers.Get([]byte{}, []byte{}, []byte("latestAccount"))
			accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
			accountID := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

			registers.Set(accountID, accountID, []byte("balance"), big.NewInt(0).Bytes())
			registers.Set(accountID, accountID, []byte("key"), key)
			registers.Set(accountID, accountID, []byte("code"), code)

			registers.Set([]byte{}, []byte{}, []byte("latestAccount"), accountID)

			return accountID, nil
		},
	}

	result, err = c.runtime.ExecuteScript(script, runtimeInterface)

	return result, registers, err
}

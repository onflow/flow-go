package execution

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// Computer uses a runtime instance to execute transactions and scripts.
type Computer struct {
	runtime       runtime.Runtime
	runtimeLogger func(string)
}

// NewComputer returns a new Computer initialized with a runtime and logger.
func NewComputer(runtime runtime.Runtime, runtimeLogger func(string)) *Computer {
	return &Computer{
		runtime:       runtime,
		runtimeLogger: runtimeLogger,
	}
}

// ExecuteTransaction executes the provided transaction in the runtime.
//
// This function initializes a new runtime context using the provided registers view, as well as
// the accounts that authorized the transaction.
//
// An error is returned if the transaction script cannot be parsed or reverts during execution.
func (c *Computer) ExecuteTransaction(registers *RegistersView, tx *types.SignedTransaction) error {
	runtimeContext := NewRuntimeContext(registers)
	runtimeContext.Accounts = []types.Address{tx.PayerSignature.Account}
	runtimeContext.Logger = c.runtimeLogger

	_, err := c.runtime.ExecuteScript(tx.Script, runtimeContext)
	return err
}

// ExecuteScript executes a plain script in the runtime.
//
// This function initializes a new runtime context using the provided registers view.
func (c *Computer) ExecuteScript(registers *RegistersView, script []byte) (interface{}, error) {
	runtimeContext := NewRuntimeContext(registers)
	runtimeContext.Logger = c.runtimeLogger

	return c.runtime.ExecuteScript(script, runtimeContext)
}

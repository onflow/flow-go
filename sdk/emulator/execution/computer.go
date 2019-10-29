package execution

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Computer uses a runtime instance to execute transactions and scripts.
type Computer struct {
	runtime        runtime.Runtime
	onLogMessage   func(string)
	onEventEmitted func(event flow.Event, blockNumber uint64, txHash crypto.Hash)
}

// NewComputer returns a new Computer initialized with a runtime and logger.
func NewComputer(
	runtime runtime.Runtime,
	onLogMessage func(string),
) *Computer {
	return &Computer{
		runtime:      runtime,
		onLogMessage: onLogMessage,
	}
}

// ExecuteTransaction executes the provided transaction in the runtime.
//
// This function initializes a new runtime context using the provided registers view, as well as
// the accounts that authorized the transaction.
//
// An error is returned if the transaction script cannot be parsed or reverts during execution.
func (c *Computer) ExecuteTransaction(registers *flow.RegistersView, tx *flow.Transaction) ([]flow.Event, error) {
	runtimeContext := NewRuntimeContext(registers)

	runtimeContext.SetSigningAccounts(tx.ScriptAccounts)
	runtimeContext.SetOnLogMessage(c.onLogMessage)

	err := c.runtime.ExecuteTransaction(tx.Script, runtimeContext, tx.Hash())
	if err != nil {
		return nil, err
	}

	return runtimeContext.Events(), nil
}

// ExecuteScript executes a plain script in the runtime.
//
// This function initializes a new runtime context using the provided registers view.
func (c *Computer) ExecuteScript(registers *flow.RegistersView, script []byte) (interface{}, []flow.Event, error) {
	runtimeContext := NewRuntimeContext(registers)
	runtimeContext.SetOnLogMessage(c.onLogMessage)

	scriptHash := ScriptHash(script)

	value, err := c.runtime.ExecuteScript(script, runtimeContext, scriptHash)
	if err != nil {
		return nil, nil, err
	}

	return value, runtimeContext.Events(), nil
}

package execution

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
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
func (c *Computer) ExecuteTransaction(registers *flow.RegistersView, tx flow.Transaction) ([]flow.Event, error) {
	runtimeContext := NewRuntimeContext(registers)

	runtimeContext.SetLogger(c.onLogMessage)
	runtimeContext.SetChecker(func(code []byte, location runtime.Location) error {
		return c.runtime.ParseAndCheckProgram(code, runtimeContext, location)
	})
	runtimeContext.SetSigningAccounts(tx.ScriptAccounts)

	location := runtime.TransactionLocation(tx.Hash())

	err := c.runtime.ExecuteTransaction(tx.Script, runtimeContext, location)
	if err != nil {
		return nil, err
	}

	events := convertEvents(runtimeContext.Events(), tx.Hash())

	return events, nil
}

// ExecuteScript executes a plain script in the runtime.
//
// This function initializes a new runtime context using the provided registers view.
func (c *Computer) ExecuteScript(registers *flow.RegistersView, script []byte) (values.Value, []flow.Event, error) {
	runtimeContext := NewRuntimeContext(registers)
	runtimeContext.SetLogger(c.onLogMessage)

	location := runtime.ScriptLocation(ScriptHash(script))

	value, err := c.runtime.ExecuteScript(script, runtimeContext, location)
	if err != nil {
		return nil, nil, err
	}

	events := convertEvents(runtimeContext.Events(), nil)

	return value, events, nil
}

func convertEvents(values []values.Event, txHash crypto.Hash) []flow.Event {
	events := make([]flow.Event, len(values))

	for i, value := range values {
		payload, err := encoding.Encode(value)
		if err != nil {
			panic("failed to encode event")
		}

		events[i] = flow.Event{
			Type:    value.Identifier,
			TxHash:  txHash,
			Index:   uint(i),
			Payload: payload,
		}
	}

	return events
}

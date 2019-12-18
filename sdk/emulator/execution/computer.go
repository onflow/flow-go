package execution

import (
	"errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	encodingValues "github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

// Computer uses a runtime instance to execute transactions and scripts.
type Computer struct {
	runtime        runtime.Runtime
	onEventEmitted func(event flow.Event, blockNumber uint64, txHash crypto.Hash)
}

// A Result is the result of executing a script or transaction.
type Result struct {
	Error  error
	Logs   []string
	Events []flow.Event
}

func (r Result) Succeeded() bool {
	return r.Error == nil
}

func (r Result) Reverted() bool {
	return !r.Succeeded()
}

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TransactionHash crypto.Hash
	Result
}

// A ScriptResult is the result of executing a script.
type ScriptResult struct {
	ScriptHash crypto.Hash
	Value      values.Value
	Result
}

// NewComputer returns a new Computer initialized with a runtime.
func NewComputer(
	runtime runtime.Runtime,
) *Computer {
	return &Computer{
		runtime: runtime,
	}
}

// ExecuteTransaction executes the provided transaction in the runtime.
//
// This function initializes a new runtime context using the provided ledger view, as well as
// the accounts that authorized the transaction.
//
// An error is returned if the transaction script cannot be parsed or reverts during execution.
func (c *Computer) ExecuteTransaction(ledger *flow.LedgerView, tx flow.Transaction) (TransactionResult, error) {
	runtimeContext := NewRuntimeContext(ledger)

	runtimeContext.SetChecker(func(code []byte, location runtime.Location) error {
		return c.runtime.ParseAndCheckProgram(code, runtimeContext, location)
	})
	runtimeContext.SetSigningAccounts(tx.ScriptAccounts)

	location := runtime.TransactionLocation(tx.Hash())

	err := c.runtime.ExecuteTransaction(tx.Script, runtimeContext, location)

	events := convertEvents(runtimeContext.Events(), tx.Hash())

	if err != nil {
		if errors.As(err, &runtime.Error{}) {
			// runtime errors occur when the execution reverts
			return TransactionResult{
				TransactionHash: tx.Hash(),
				Result: Result{
					Error:  err,
					Logs:   runtimeContext.Logs(),
					Events: events,
				},
			}, nil
		}

		// other errors are unexpected and should be treated as fatal
		return TransactionResult{}, err
	}

	return TransactionResult{
		TransactionHash: tx.Hash(),
		Result: Result{
			Error:  nil,
			Logs:   runtimeContext.Logs(),
			Events: events,
		},
	}, nil
}

// ExecuteScript executes a plain script in the runtime.
//
// This function initializes a new runtime context using the provided registers view.
func (c *Computer) ExecuteScript(view *flow.LedgerView, script []byte) (ScriptResult, error) {
	runtimeContext := NewRuntimeContext(view)

	scriptHash := ScriptHash(script)

	location := runtime.ScriptLocation(scriptHash)

	value, err := c.runtime.ExecuteScript(script, runtimeContext, location)

	events := convertEvents(runtimeContext.Events(), nil)

	if err != nil {
		if errors.As(err, &runtime.Error{}) {
			// runtime errors occur when the execution reverts
			return ScriptResult{
				ScriptHash: scriptHash,
				Value:      value,
				Result: Result{
					Error:  err,
					Logs:   runtimeContext.Logs(),
					Events: events,
				},
			}, nil
		}

		// other errors are unexpected and should be treated as fatal
		return ScriptResult{}, err
	}

	return ScriptResult{
		ScriptHash: scriptHash,
		Value:      value,
		Result: Result{
			Error:  nil,
			Logs:   runtimeContext.Logs(),
			Events: events,
		},
	}, nil
}

func convertEvents(values []values.Event, txHash crypto.Hash) []flow.Event {
	events := make([]flow.Event, len(values))

	for i, value := range values {
		payload, err := encodingValues.Encode(value)
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

package emulator

import (
	"errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
	encodingValues "github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/emulator/execution"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
)

// A computer uses a runtime instance to execute transactions and scripts.
type computer struct {
	runtime        runtime.Runtime
	onEventEmitted func(event flow.Event, blockNumber uint64, txHash crypto.Hash)
}

// newComputer returns a new computer initialized with a runtime.
func newComputer(
	runtime runtime.Runtime,
) *computer {
	return &computer{
		runtime: runtime,
	}
}

// ExecuteTransaction executes the provided transaction in the runtime.
//
// This function initializes a new runtime context using the provided ledger view, as well as
// the accounts that authorized the transaction.
//
// An error is returned if the transaction script cannot be parsed or reverts during execution.
func (c *computer) ExecuteTransaction(ledger *types.LedgerView, tx flow.Transaction) (TransactionResult, error) {
	runtimeContext := execution.NewRuntimeContext(ledger)

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
				Error:           err,
				Logs:            runtimeContext.Logs(),
				Events:          events,
			}, nil
		}

		// other errors are unexpected and should be treated as fatal
		return TransactionResult{}, err
	}

	return TransactionResult{
		TransactionHash: tx.Hash(),
		Error:           nil,
		Logs:            runtimeContext.Logs(),
		Events:          events,
	}, nil
}

// ExecuteScript executes a plain script in the runtime.
//
// This function initializes a new runtime context using the provided registers view.
func (c *computer) ExecuteScript(view *types.LedgerView, script []byte) (ScriptResult, error) {
	runtimeContext := execution.NewRuntimeContext(view)

	scriptHash := hash.DefaultHasher.ComputeHash(script)

	location := runtime.ScriptLocation(scriptHash)

	value, err := c.runtime.ExecuteScript(script, runtimeContext, location)

	events := convertEvents(runtimeContext.Events(), nil)

	if err != nil {
		if errors.As(err, &runtime.Error{}) {
			// runtime errors occur when the execution reverts
			return ScriptResult{
				ScriptHash: scriptHash,
				Value:      value,
				Error:      err,
				Logs:       runtimeContext.Logs(),
				Events:     events,
			}, nil
		}

		// other errors are unexpected and should be treated as fatal
		return ScriptResult{}, err
	}

	return ScriptResult{
		ScriptHash: scriptHash,
		Value:      value,
		Error:      nil,
		Logs:       runtimeContext.Logs(),
		Events:     events,
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

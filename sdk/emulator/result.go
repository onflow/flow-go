package emulator

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TransactionHash crypto.Hash
	Error           error
	Logs            []string
	Events          []flow.Event
}

// Succeeded returns true if the transaction executed without errors.
func (r TransactionResult) Succeeded() bool {
	return r.Error == nil
}

// Reverted returns true if the transaction executed with errors.
func (r TransactionResult) Reverted() bool {
	return !r.Succeeded()
}

// A ScriptResult is the result of executing a script.
type ScriptResult struct {
	ScriptHash crypto.Hash
	Value      values.Value
	Error      error
	Logs       []string
	Events     []flow.Event
}

// Succeeded returns true if the script executed without errors.
func (r ScriptResult) Succeeded() bool {
	return r.Error == nil
}

// Reverted returns true if the script executed with errors.
func (r ScriptResult) Reverted() bool {
	return !r.Succeeded()
}

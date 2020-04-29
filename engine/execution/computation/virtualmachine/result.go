package virtualmachine

import (
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TransactionID flow.Identifier
	Events        []runtime.Event
	Logs          []string
	Error         error
	GasUsed       uint64
}

func (r TransactionResult) Succeeded() bool {
	return r.Error == nil
}

// A ScriptResult is the result of executing a script.
type ScriptResult struct {
	ScriptID flow.Identifier
	Value    runtime.Value
	Logs     []string
	Error    error
	Events   []runtime.Event
}

func (r ScriptResult) Succeeded() bool {
	return r.Error == nil
}

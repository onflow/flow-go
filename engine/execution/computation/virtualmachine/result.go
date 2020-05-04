package virtualmachine

import (
	"github.com/onflow/cadence"

	"github.com/dapperlabs/flow-go/model/flow"
)

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TransactionID flow.Identifier
	Events        []cadence.Event
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
	Value    cadence.Value
	Logs     []string
	Error    error
}

func (r ScriptResult) Succeeded() bool {
	return r.Error == nil
}

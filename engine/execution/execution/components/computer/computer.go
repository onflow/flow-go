package computer

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/execution/modules/context"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TxID      flow.Identifier
	Succeeded bool
	Events    []*flow.Event
	GasUsed   uint64
}

// A Computer uses the Cadence runtime to compute transaction results.
type Computer struct {
	runtime         runtime.Runtime
	contextProvider context.Provider
}

// New initializes a new computer with a runtime and context provider.
func New(runtime runtime.Runtime, contextProvider context.Provider) *Computer {
	return &Computer{
		runtime:         runtime,
		contextProvider: contextProvider,
	}
}

// ComputeTransaction computes the result of a transaction.
func (c *Computer) ComputeTransaction(tx *flow.Transaction) (*TransactionResult, error) {
	return nil, nil
}

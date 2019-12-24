package executor

import "github.com/dapperlabs/flow-go/model/flow"

// An Executor executes the transactions in a block.
type Executor struct{}

// New creates a new Executor.
func New() *Executor {
	return &Executor{}
}

// ExecuteBlock executes a block and returns an execution receipt.
func (e *Executor) ExecuteBlock(
	header *flow.Header,
	collections []*flow.Collection,
) ([]*flow.ExecutionReceipt, error) {
	panic("implement me!")
	return nil, nil
}

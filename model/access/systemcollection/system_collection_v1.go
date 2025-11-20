package systemcollection

import (
	_ "embed"
	"fmt"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// We copy implementations from the blueprints package to freeze each versioned system
// collection at a specific point in time. This prevents future changes to blueprints
// from automatically affecting historical versions, ensuring version stability.
//
// When creating a new version, create a new versioned type and copy the implementation
// from blueprints at that point in time. Include all constants by hard coding their values.
// Create new static template files in the scripts directory if any of the templates changed.

// builderV1 is the latest and current version of the system collection.
type builderV1 struct{}

// ProcessCallbacksTransaction constructs a transaction for processing callbacks, for the given callback.
//
// No error returns are expected during normal operation.
func (b *builderV1) ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return blueprints.ProcessCallbacksTransaction(chain)
}

// ExecuteCallbacksTransactions constructs a list of transaction to execute callbacks, for the given chain.
//
// No error returns are expected during normal operation.
func (b *builderV1) ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	return blueprints.ExecuteCallbacksTransactions(chain, processEvents)
}

// SystemChunkTransaction creates and returns the transaction corresponding to the
// system chunk for the given chain.
//
// No error returns are expected during normal operation.
func (b *builderV1) SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return blueprints.SystemChunkTransaction(chain)
}

// SystemCollection constructs a system collection for the given chain.
// Uses the provided event provider to get events required to construct the system collection.
// A nil event provider behaves the same as an event provider that returns an empty EventsList.
//
// No error returns are expected during normal operation.
func (b *builderV1) SystemCollection(chain flow.Chain, providerFn access.EventProvider) (*flow.Collection, error) {
	process, err := b.ProcessCallbacksTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct process callbacks transaction: %w", err)
	}

	var processEvents flow.EventsList
	if providerFn != nil {
		processEvents, err = providerFn()
		if err != nil {
			return nil, fmt.Errorf("failed to get process transactions events: %w", err)
		}
	}

	executes, err := b.ExecuteCallbacksTransactions(chain, processEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to construct execute callbacks transactions: %w", err)
	}

	systemTx, err := b.SystemChunkTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
	}

	transactions := make([]*flow.TransactionBody, 0, len(executes)+2) // +2 process and system tx
	transactions = append(transactions, process)
	transactions = append(transactions, executes...)
	transactions = append(transactions, systemTx)

	collection, err := flow.NewCollection(flow.UntrustedCollection{
		Transactions: transactions,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to construct system collection: %w", err)
	}

	return collection, nil
}

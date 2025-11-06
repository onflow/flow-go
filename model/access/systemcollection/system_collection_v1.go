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
// from blueprints at that point in time.

// builderV1 is the latest and current version of the system collection.
type builderV1 struct{}

func (s *builderV1) ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return blueprints.ProcessCallbacksTransaction(chain)
}

func (s *builderV1) ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	return blueprints.ExecuteCallbacksTransactions(chain, processEvents)
}

func (s *builderV1) SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return blueprints.SystemChunkTransaction(chain)
}

func (s *builderV1) SystemCollection(chain flow.Chain, providerFn access.EventProvider) (*flow.Collection, error) {
	process, err := s.ProcessCallbacksTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct process callbacks transaction: %w", err)
	}

	processEvents, err := providerFn()
	if err != nil {
		return nil, fmt.Errorf("failed to get process events: %w", err)
	}

	executes, err := s.ExecuteCallbacksTransactions(chain, processEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to construct execute callbacks transactions: %w", err)
	}

	systemTx, err := s.SystemChunkTransaction(chain)
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

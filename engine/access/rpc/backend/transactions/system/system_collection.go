package system

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/flow"
)

// SystemCollection represents a system collection and exposes the transaction bodies of each transaction
// within the collection.
type SystemCollection struct {
	collection *flow.Collection
	lookup     map[flow.Identifier]*flow.TransactionBody
	systemTxID flow.Identifier
}

// DefaultSystemCollection returns the default system collection for the given chain ID.
// This is the system collection that contains only static system transactions, and no scheduled transactions.
// If scheduled transactions are disabled, the system collection will contain only the system chunk transaction.
//
// No error returns are expected during normal operation.
func DefaultSystemCollection(chainID flow.ChainID, scheduledTransactionsEnabled bool) (*SystemCollection, error) {
	if scheduledTransactionsEnabled {
		return NewSystemCollection(chainID, nil)
	}

	systemTx, err := blueprints.SystemChunkTransaction(chainID.Chain())
	if err != nil {
		return nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
	}
	systemTxID := systemTx.ID()

	return &SystemCollection{
		collection: &flow.Collection{
			Transactions: []*flow.TransactionBody{systemTx},
		},
		lookup: map[flow.Identifier]*flow.TransactionBody{
			systemTxID: systemTx,
		},
		systemTxID: systemTxID,
	}, nil
}

// NewSystemCollection returns a new system collection for the given chain ID including scheduled
// transactions for each PendingExecution event contained in the events list.
//
// No error returns are expected during normal operation.
func NewSystemCollection(chainID flow.ChainID, events flow.EventsList) (*SystemCollection, error) {
	systemCollection, err := blueprints.SystemCollection(chainID.Chain(), events)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
	}

	if len(systemCollection.Transactions) < 2 {
		return nil, fmt.Errorf("expected 2 transactions in system collection, got %d", len(systemCollection.Transactions))
	}

	lookup := make(map[flow.Identifier]*flow.TransactionBody, len(systemCollection.Transactions))
	for _, tx := range systemCollection.Transactions {
		lookup[tx.ID()] = tx
	}

	systemTxIndex := len(systemCollection.Transactions) - 1
	return &SystemCollection{
		collection: systemCollection,
		lookup:     lookup,
		systemTxID: systemCollection.Transactions[systemTxIndex].ID(),
	}, nil
}

// ByID returns the system transaction body by ID.
// Returns true if the transaction was found in the collection, false otherwise.
func (s *SystemCollection) ByID(id flow.Identifier) (*flow.TransactionBody, bool) {
	tx, ok := s.lookup[id]
	return tx, ok
}

// Transactions returns the transactions in the system collection.
func (s *SystemCollection) Transactions() []*flow.TransactionBody {
	return s.collection.Transactions
}

// SystemTxID returns the ID of the system transaction.
// This is the last transaction in the system collection, which is responsible for protocol management.
func (s *SystemCollection) SystemTxID() flow.Identifier {
	return s.systemTxID
}

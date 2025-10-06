package inmemory

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type TransactionsReader struct {
	transactions map[flow.Identifier]flow.TransactionBody
}

var _ storage.TransactionsReader = (*TransactionsReader)(nil)

func NewTransactions(transactions []*flow.TransactionBody) *TransactionsReader {
	transactionsMap := make(map[flow.Identifier]flow.TransactionBody)
	for _, transaction := range transactions {
		transactionsMap[transaction.ID()] = *transaction
	}

	return &TransactionsReader{
		transactions: transactionsMap,
	}
}

// ByID returns the transaction for the given fingerprint.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no transaction with provided ID was found.
func (t *TransactionsReader) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	val, ok := t.transactions[txID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

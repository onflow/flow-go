package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Transactions struct {
	db *badger.DB
}

func NewTransactions(db *badger.DB) *Transactions {
	t := Transactions{
		db: db,
	}
	return &t
}

func (t *Transactions) Store(tx *flow.TransactionBody) error {
	err := operation.RetryOnConflict(t.db.Update, operation.InsertTransaction(tx))
	if err != nil {
		return fmt.Errorf("could not insert transaction: %w", err)
	}
	return nil
}

func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {

	var tx flow.TransactionBody
	err := t.db.View(operation.RetrieveTransaction(txID, &tx))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve transaction: %w", err)
	}
	return &tx, nil
}

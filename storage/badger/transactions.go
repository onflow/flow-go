package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
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

func (t *Transactions) ByFingerprint(fp flow.Fingerprint) (*flow.Transaction, error) {
	var tx flow.Transaction

	err := t.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveTransaction(fp, &tx)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve transaction: %w", err)
		}
		return nil
	})

	return &tx, err
}

func (t *Transactions) Insert(tx *flow.Transaction) error {
	return t.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertTransaction(tx.Fingerprint(), tx)(btx)
		if err != nil {
			return fmt.Errorf("could not insert transaction: %w", err)
		}
		return nil
	})
}

func (t *Transactions) Remove(hash crypto.Hash) error {
	return t.db.Update(func(btx *badger.Txn) error {
		err := operation.RemoveTransaction(hash)(btx)
		if err != nil {
			return fmt.Errorf("could not remove transaction: %w", err)
		}
		return nil
	})
}

package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"

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

func (t *Transactions) ByHash(hash crypto.Hash) (*flow.Transaction, error) {
	var tx flow.Transaction

	err := t.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveTransaction(hash, &tx)(btx)
		if err != nil {
			return errors.Wrap(err, "could not retrieve transaction")
		}
		return nil
	})

	return &tx, err
}

func (t *Transactions) Insert(tx *flow.Transaction) error {
	return t.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertTransaction(tx.Hash(), tx)(btx)
		if err != nil {
			return errors.Wrap(err, "could not insert transaction")
		}
		return nil
	})
}

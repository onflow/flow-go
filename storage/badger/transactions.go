package badger

import (
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"
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

func (t *Transactions) ByFingerprint(fp model.Fingerprint) (*flow.Transaction, error) {
	var tx flow.Transaction

	err := t.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveTransaction(fp, &tx)(btx)
		if err != nil {
			return errors.Wrap(err, "could not retrieve transaction")
		}
		return nil
	})

	return &tx, err
}

func (t *Transactions) Insert(tx *flow.Transaction) error {
	return t.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertTransaction(tx.Fingerprint(), tx)(btx)
		if err != nil {
			return errors.Wrap(err, "could not insert transaction")
		}
		return nil
	})
}

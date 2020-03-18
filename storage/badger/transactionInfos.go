package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type TransactionInfos struct {
	db *badger.DB
}

func NewTransactionInfo(db *badger.DB) *TransactionInfos {
	t := TransactionInfos{
		db: db,
	}
	return &t
}

func (t *TransactionInfos) Store(tx *flow.TransactionInfo) error {
	return t.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertTransactionInfo(tx)(btx)
		if err != nil {
			return fmt.Errorf("could not insert transaction info: %w", err)
		}
		return nil
	})
}

func (t *TransactionInfos) ByID(txID flow.Identifier) (*flow.TransactionInfo, error) {

	var tx flow.TransactionInfo
	err := t.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveTransactionInfo(txID, &tx)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve transaction info: %w", err)
		}
		return nil
	})

	return &tx, err
}

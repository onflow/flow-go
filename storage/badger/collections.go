package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Collections struct {
	db *badger.DB
}

func NewCollections(db *badger.DB) *Collections {
	c := Collections{
		db: db,
	}
	return &c
}

func (c Collections) ByFingerprint(hash flow.Fingerprint) (*flow.GuaranteedCollection, error) {

}

func (c Collections) ByFingerprintWithTransactions(hash flow.Fingerprint) ([]*flow.Transaction, error) {
	panic("implement me")
}

func (c Collections) Insert(gc *flow.GuaranteedCollection) error {
	return c.db.Update(func (tx *badger.Txn) error {
		err := operation.InsertCollection(gc.Fingerprint(), gc)(tx)
		if err != nil {
			return fmt.Errorf("could not insert guaranteed collection: %w", err)
		}
		return nil
	})
}

func (c Collections) Remove(hash flow.Fingerprint) error {
	return c.db.Update(func (tx *badger.Txn) error {
		err := operation.RemoveCollection(hash)(tx)
		if err != nil {
			return fmt.Errorf("could not remove guaranteed collection: %w", err)
		}
		return nil
	})
}

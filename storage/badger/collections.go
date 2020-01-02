package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Collections struct {
	db *badger.DB
}

func NewCollections(db *badger.DB) *Collections {
	b := &Collections{
		db: db,
	}
	return b
}

func (c *Collections) ByFingerprint(hash flow.Fingerprint) (*flow.Collection, error) {

	var collection flow.Collection

	err := c.db.View(func(tx *badger.Txn) error {

		err := operation.RetrieveFlowCollection(hash, &collection)(tx)

		if err != nil {
			if err == storage.NotFoundErr {
				return err
			}
			return errors.Wrap(err, "could not retrieve flow collection")
		}

		return nil
	})

	return &collection, err
}

func (c *Collections) Save(collection *flow.Collection) error {
	return c.db.Update(func(tx *badger.Txn) error {
		err := operation.PersistFlowCollection(collection)(tx)
		if err != nil {
			return errors.Wrap(err, "could not insert flow collection")
		}
		return nil
	})
}

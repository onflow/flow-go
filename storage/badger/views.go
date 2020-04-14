package badger

import (
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Views implements a simple read-only block storage around a badger DB.
type Views struct {
	db *badger.DB
}

func NewViews(db *badger.DB) *Views {
	b := &Views{
		db: db,
	}
	return b
}

func (b *Views) StoreLatest(view uint64) error {
	err := b.db.Update(func(tx *badger.Txn) error {
		err := operation.UpdateView(view)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			return operation.InsertView(view)(tx)
		}
		return err
	})
	return err
}

func (b *Views) RetrieveLatest() (uint64, error) {
	var view uint64
	err := b.db.View(func(tx *badger.Txn) error {
		err := operation.RetrieveView(&view)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			view = 1
			return nil
		}
		return err
	})
	return view, err
}

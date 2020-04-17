package badger

import (
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

const (
	started = 0 // prefix for started view
	voted   = 1 // prefix for voted view
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

func (b *Views) StoreStarted(view uint64) error {
	err := b.db.Update(func(tx *badger.Txn) error {
		err := operation.UpdateView(started, view)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			return operation.InsertView(started, view)(tx)
		}
		return err
	})
	return err
}

func (b *Views) RetrieveStarted() (uint64, error) {
	var view uint64
	err := b.db.View(func(tx *badger.Txn) error {
		err := operation.RetrieveView(started, &view)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			view = 0
			return nil
		}
		return err
	})
	return view, err
}

func (b *Views) StoreVoted(view uint64) error {
	err := b.db.Update(func(tx *badger.Txn) error {
		err := operation.UpdateView(voted, view)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			return operation.InsertView(voted, view)(tx)
		}
		return err
	})
	return err
}

func (b *Views) RetrieveVoted() (uint64, error) {
	var view uint64
	err := b.db.View(func(tx *badger.Txn) error {
		err := operation.RetrieveView(voted, &view)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			view = 0
			return nil
		}
		return err
	})
	return view, err
}

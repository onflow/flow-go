package operation

import (
	"github.com/dgraph-io/badger/v2"
)

// InsertView inserts a view into the database.
func InsertView(prefix uint8, view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeView, prefix), view)
}

// UpdateView updates the view in the database.
func UpdateView(prefix uint8, view uint64) func(*badger.Txn) error {
	return update(makePrefix(codeView, prefix), view)
}

// RetrieveView retrieves a view from the database.
func RetrieveView(prefix uint8, view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeView, prefix), view)
}

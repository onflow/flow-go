package operation

import (
	"github.com/dgraph-io/badger/v2"
)

// InsertView inserts a view into the database.
func InsertView(action uint8, view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeView, action), view)
}

// UpdateView updates the view in the database.
func UpdateView(action uint8, view uint64) func(*badger.Txn) error {
	return update(makePrefix(codeView, action), view)
}

// RetrieveView retrieves a view from the database.
func RetrieveView(action uint8, view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeView, action), view)
}

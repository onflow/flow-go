package operation

import (
	"github.com/dgraph-io/badger/v2"
)

// InsertView inserts a view into the database.
func InsertView(view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeView), view)
}

// UpdateView updates the view in the database.
func UpdateView(view uint64) func(*badger.Txn) error {
	return update(makePrefix(codeView), view)
}

// RetrieveView retrieves a view from the database.
func RetrieveView(view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeView), view)
}

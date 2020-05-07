package operation

import (
	"github.com/dgraph-io/badger/v2"
)

// InsertStartedView inserts a view into the database.
func InsertStartedView(view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeStartedView), view)
}

// UpdateStartedView updates the view in the database.
func UpdateStartedView(view uint64) func(*badger.Txn) error {
	return update(makePrefix(codeStartedView), view)
}

// RetrieveStartedView retrieves a view from the database.
func RetrieveStartedView(view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeStartedView), view)
}

// InsertVotedView inserts a view into the database.
func InsertVotedView(view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeVotedView), view)
}

// UpdateVotedView updates the view in the database.
func UpdateVotedView(view uint64) func(*badger.Txn) error {
	return update(makePrefix(codeVotedView), view)
}

// RetrieveVotedView retrieves a view from the database.
func RetrieveVotedView(view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeVotedView), view)
}

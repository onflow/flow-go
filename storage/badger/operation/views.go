package operation

import (
	"github.com/dgraph-io/badger/v2"
)

// InsertStartedView inserts a view into the database.
func InsertStartedView(view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeStartedView), view)
}

// UpdateStartedView updates the view in the database.
func UpdateStartedView(chainID string, view uint64) func(*badger.Txn) error {
	return update(makePrefix(codeStartedView, chainID), view)
}

// RetrieveStartedView retrieves a view from the database.
func RetrieveStartedView(chainID string, view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeStartedView, chainID), view)
}

// InsertVotedView inserts a view into the database.
func InsertVotedView(chainID string, view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeVotedView, chainID), view)
}

// UpdateVotedView updates the view in the database.
func UpdateVotedView(chainID string, view uint64) func(*badger.Txn) error {
	return update(makePrefix(codeVotedView, chainID), view)
}

// RetrieveVotedView retrieves a view from the database.
func RetrieveVotedView(chainID string, view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeVotedView, chainID), view)
}

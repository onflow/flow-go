package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertStartedView inserts a view into the database.
func InsertStartedView(chainID flow.ChainID, view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeStartedView, chainID), view)
}

// UpdateStartedView updates the view in the database.
func UpdateStartedView(chainID flow.ChainID, view uint64) func(*badger.Txn) error {
	return update(makePrefix(codeStartedView, chainID), view)
}

// RetrieveStartedView retrieves a view from the database.
func RetrieveStartedView(chainID flow.ChainID, view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeStartedView, chainID), view)
}

// InsertVotedView inserts a view into the database.
func InsertVotedView(chainID flow.ChainID, view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeVotedView, chainID), view)
}

// UpdateVotedView updates the view in the database.
func UpdateVotedView(chainID flow.ChainID, view uint64) func(*badger.Txn) error {
	return update(makePrefix(codeVotedView, chainID), view)
}

// RetrieveVotedView retrieves a view from the database.
func RetrieveVotedView(chainID flow.ChainID, view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeVotedView, chainID), view)
}

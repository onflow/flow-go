package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertEpochProtocolState inserts an epoch protocol state entry by ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func InsertEpochProtocolState(entryID flow.Identifier, entry *flow.MinEpochStateEntry) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochProtocolState, entryID), entry)
}

// RetrieveEpochProtocolState retrieves an epoch protocol state entry by ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func RetrieveEpochProtocolState(entryID flow.Identifier, entry *flow.MinEpochStateEntry) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochProtocolState, entryID), entry)
}

// IndexEpochProtocolState indexes an epoch protocol state entry by block ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func IndexEpochProtocolState(blockID flow.Identifier, epochProtocolStateEntryID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochProtocolStateByBlockID, blockID), epochProtocolStateEntryID)
}

// LookupEpochProtocolState finds an epoch protocol state entry ID by block ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupEpochProtocolState(blockID flow.Identifier, epochProtocolStateEntryID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochProtocolStateByBlockID, blockID), epochProtocolStateEntryID)
}

package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertEpochProtocolState inserts an epoch protocol state entry by ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func InsertEpochProtocolState(w storage.Writer, entryID flow.Identifier, entry *flow.MinEpochStateEntry) error {
	return UpsertByKey(w, MakePrefix(codeEpochProtocolState, entryID), entry)
}

// RetrieveEpochProtocolState retrieves an epoch protocol state entry by ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func RetrieveEpochProtocolState(r storage.Reader, entryID flow.Identifier, entry *flow.MinEpochStateEntry) error {
	return RetrieveByKey(r, MakePrefix(codeEpochProtocolState, entryID), entry)
}

// IndexEpochProtocolState indexes an epoch protocol state entry by block ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func IndexEpochProtocolState(w storage.Writer, blockID flow.Identifier, epochProtocolStateEntryID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeEpochProtocolStateByBlockID, blockID), epochProtocolStateEntryID)
}

// LookupEpochProtocolState finds an epoch protocol state entry ID by block ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupEpochProtocolState(r storage.Reader, blockID flow.Identifier, epochProtocolStateEntryID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeEpochProtocolStateByBlockID, blockID), epochProtocolStateEntryID)
}

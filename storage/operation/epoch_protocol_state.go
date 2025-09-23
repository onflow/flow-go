package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertEpochProtocolState inserts an epoch protocol state entry by ID.
// Error returns:
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
//
// CAUTION:
//   - The caller must acquire the lock [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// Error returns:
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func IndexEpochProtocolState(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, epochProtocolStateEntryID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	return UpsertByKey(w, MakePrefix(codeEpochProtocolStateByBlockID, blockID), epochProtocolStateEntryID)
}

// LookupEpochProtocolState finds an epoch protocol state entry ID by block ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupEpochProtocolState(r storage.Reader, blockID flow.Identifier, epochProtocolStateEntryID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeEpochProtocolStateByBlockID, blockID), epochProtocolStateEntryID)
}

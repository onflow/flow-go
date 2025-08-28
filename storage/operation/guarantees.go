package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UnsafeInsertGuarantee inserts a [flow.CollectionGuarantee] into the database, keyed by the collection ID.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exists.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No other errors are expected during normal operation.
func UnsafeInsertGuarantee(lctx lockctx.Proof, w storage.Writer, collID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot insert guarantee %s for collection %s without holding lock %s",
			guarantee.ID(), collID, storage.LockInsertBlock)
	}

	return UpsertByKey(w, MakePrefix(codeGuarantee, collID), guarantee)
}

// RetrieveGuarantee retrieves a [flow.CollectionGuarantee] by the collection ID.
// For every collection that has been guaranteed, this data should be populated.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `collID` does not refer to a known guaranteed collection
func RetrieveGuarantee(r storage.Reader, collID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	return RetrieveByKey(r, MakePrefix(codeGuarantee, collID), guarantee)
}

// IndexPayloadGuarantees indexes the list of collection guarantees that were included in the specified block,
// keyed by the block ID. It produces a mapping from block ID to the list of collection guarantees contained in
// the block's payload. The collection guarantees are represented by their respective IDs.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No other errors are expected during normal operation.
func IndexPayloadGuarantees(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, guarIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index guarantee for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadGuarantees, blockID), guarIDs)
}

// LookupPayloadGuarantees retrieves the list of guarantee IDs that were included in the payload
// of the specified block. For every known block (at or above the root block height), this index should be populated.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a known block
func LookupPayloadGuarantees(r storage.Reader, blockID flow.Identifier, guarIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadGuarantees, blockID), guarIDs)
}

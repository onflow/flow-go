package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertGuarantee inserts a collection guarantee by ID.
//
// CAUTION: The caller must ensure guaranteeID is a collision-resistant hash of the provided
// guarantee! This method silently overrides existing data, which is safe only if for the same
// key, we always write the same value.
//
// No errors expected during normal operations.
func InsertGuarantee(w storage.Writer, guaranteeID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	return UpsertByKey(w, MakePrefix(codeGuarantee, guaranteeID), guarantee)
}

// IndexGuarantee inserts a [flow.CollectionGuarantee] into the database, keyed by the collection ID.
//
// Expected errors during normal operations:
//   - [storage.ErrDataMismatch] if a different [flow.CollectionGuarantee] has already been indexed for the given collection ID.
//   - All other errors have to be treated as unexpected failures from the database layer.
func IndexGuarantee(lctx lockctx.Proof, rw storage.ReaderBatchWriter, collectionID flow.Identifier, guaranteeID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index guarantee for collectionID %v without holding lock %s",
			collectionID, storage.LockInsertBlock)
	}

	var storedGuaranteeID flow.Identifier
	err := LookupGuarantee(rw.GlobalReader(), collectionID, &storedGuaranteeID)
	if err == nil {
		if storedGuaranteeID != guaranteeID {
			return fmt.Errorf("new guarantee %x did not match already stored guarantee %x, for collection %x: %w",
				guaranteeID, storedGuaranteeID, collectionID, storage.ErrDataMismatch)
		}
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed to retrieve existing guarantee for collection %x: %w", collectionID, err)
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeGuaranteeByCollectionID, collectionID), guaranteeID)
}

// RetrieveGuarantee retrieves a [flow.CollectionGuarantee] by the collection ID.
// For every collection that has been guaranteed, this data should be populated.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `collID` does not refer to a known guaranteed collection
//   - All other errors have to be treated as unexpected failures from the database layer.
func RetrieveGuarantee(r storage.Reader, collID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	return RetrieveByKey(r, MakePrefix(codeGuarantee, collID), guarantee)
}

// LookupGuarantee finds collection guarantee ID by collection ID.
// Error returns:
//   - [storage.ErrNotFound] if the key does not exist in the database
//   - All other errors have to be treated as unexpected failures from the database layer.
func LookupGuarantee(r storage.Reader, collectionID flow.Identifier, guaranteeID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeGuaranteeByCollectionID, collectionID), guaranteeID)
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
// No errors expected during normal operations.
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
//   - All other errors have to be treated as unexpected failures from the database layer.
func LookupPayloadGuarantees(r storage.Reader, blockID flow.Identifier, guarIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadGuarantees, blockID), guarIDs)
}

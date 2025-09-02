package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertGuarantee inserts a collection guarantee by ID.
// Error returns:
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func InsertGuarantee(w storage.Writer, guaranteeID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	return UpsertByKey(w, MakePrefix(codeGuarantee, guaranteeID), guarantee)
}

// IndexGuarantee inserts a [flow.CollectionGuarantee] into the database, keyed by the collection ID.
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
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer
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
func RetrieveGuarantee(r storage.Reader, collID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	return RetrieveByKey(r, MakePrefix(codeGuarantee, collID), guarantee)
}

// LookupGuarantee finds collection guarantee ID by collection ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupGuarantee(r storage.Reader, collectionID flow.Identifier, guaranteeID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeGuaranteeByCollectionID, collectionID), guaranteeID)
}

// IndexPayloadGuarantees indexes a collection guarantees by block ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer
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
//   - generic error in case of unexpected failure from the database layer
func LookupPayloadGuarantees(r storage.Reader, blockID flow.Identifier, guarIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadGuarantees, blockID), guarIDs)
}

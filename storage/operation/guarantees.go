package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertGuarantee inserts a collection guarantee into the database.
func InsertGuarantee(w storage.Writer, guaranteeID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	return UpsertByKey(w, MakePrefix(codeGuarantee, guaranteeID), guarantee)
}

// RetrieveGuarantee retrieves a collection guarantee by ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func RetrieveGuarantee(r storage.Reader, guaranteeID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	return RetrieveByKey(r, MakePrefix(codeGuarantee, guaranteeID), guarantee)
}

// IndexGuarantee indexes a collection guarantee by collection ID.
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
	// Only need to check if the lock is held, no need to check if is already stored,
	// because the duplication check is done when storing a header, which is in the same
	// batch update and holding the same lock.

	return UpsertByKey(w, MakePrefix(codePayloadGuarantees, blockID), guarIDs)
}

// LookupPayloadGuarantees finds collection guarantees by block ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupPayloadGuarantees(r storage.Reader, blockID flow.Identifier, guarIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadGuarantees, blockID), guarIDs)
}

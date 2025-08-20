package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UnsafeInsertGuarantee inserts a collection guarantee into the database.
// It's called unsafe because it doesn't check if a different guarantee was already inserted
// for the same collection ID.
func UnsafeInsertGuarantee(lctx lockctx.Proof, w storage.Writer, collID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot insert guarantee %s for collection %s without holding lock %s",
			guarantee.ID(), collID, storage.LockInsertBlock)
	}

	return UpsertByKey(w, MakePrefix(codeGuarantee, collID), guarantee)
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
func IndexGuarantee(w storage.Writer, collectionID flow.Identifier, guaranteeID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeGuaranteeByCollectionID, collectionID), guaranteeID)
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

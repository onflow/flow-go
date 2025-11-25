package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// NOTE: These insert light collections, which only contain references
// to the constituent transactions. They do not modify transactions contained
// by the collections.

// UpsertCollection inserts a [flow.LightCollection] into the storage, keyed by its ID.
//
// If the collection already exists, it will be overwritten. Note that here, the key (collection ID) is derived
// from the value (collection) via a collision-resistant hash function. Hence, unchecked overwrites pose no risk
// of data corruption, because for the same key, we expect the same value.
//
// No errors are expected during normal operation.
func UpsertCollection(w storage.Writer, collection *flow.LightCollection) error {
	return UpsertByKey(w, MakePrefix(codeCollection, collection.ID()), collection)
}

// RetrieveCollection retrieves a [flow.LightCollection] by its ID.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no collection with the specified ID is known.
func RetrieveCollection(r storage.Reader, collID flow.Identifier, collection *flow.LightCollection) error {
	return RetrieveByKey(r, MakePrefix(codeCollection, collID), collection)
}

// RemoveCollection removes a collection from the storage.
// CAUTION: this is for recovery purposes only, and should not be used during normal operations!
// It returns nil if the collection does not exist.
// No errors are expected during normal operation.
func RemoveCollection(w storage.Writer, collID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeCollection, collID))
}

// IndexCollectionPayload populates the map from a cluster block ID to the batch of transactions it contains.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertOrFinalizeClusterBlock] and hold it until the database write has been
//     committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No errors are expected during normal operation.
func IndexCollectionPayload(lctx lockctx.Proof, w storage.Writer, clusterBlockID flow.Identifier, txIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}
	return UpsertByKey(w, MakePrefix(codeIndexCollection, clusterBlockID), txIDs)
}

// LookupCollectionPayload retrieves the list of transaction IDs that constitute the payload of the specified cluster block.
// For every known cluster block, this index should be populated.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `clusterBlockID` does not refer to a known cluster block
func LookupCollectionPayload(r storage.Reader, clusterBlockID flow.Identifier, txIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexCollection, clusterBlockID), txIDs)
}

// RemoveCollectionPayloadIndices removes a collection id indexed by a block id.
// CAUTION: this is for recovery purposes only, and should not be used during normal operations!
// It returns nil if the collection does not exist.
// No errors are expected during normal operation.
func RemoveCollectionPayloadIndices(w storage.Writer, collID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexCollection, collID))
}

// IndexCollectionByTransaction indexes the given collection ID, keyed by the transaction ID.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertCollection] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// WARNING, this index is NOT BFT in its current form:
// Honest clusters ensure a transaction can only belong to one collection. However, in rare
// cases, the collector clusters can exceed byzantine thresholds -- making it possible to
// produce multiple finalized collections (aka guaranteed collections) containing the same
// transaction repeatedly.
// TODO: eventually we need to handle Byzantine clusters
//
// No errors are expected during normal operation.
func IndexCollectionByTransaction(lctx lockctx.Proof, w storage.Writer, txID flow.Identifier, collectionID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertCollection) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertCollection)
	}

	return UpsertByKey(w, MakePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

// LookupCollectionByTransaction retrieves the collection ID for the collection that contains the specified transaction.
// For every known transaction, this index should be populated.
//
// WARNING, this index is NOT BFT in its current form:
// Honest clusters ensure a transaction can only belong to one collection. However, in rare
// cases, the collector clusters can exceed byzantine thresholds -- making it possible to
// produce multiple finalized collections (aka guaranteed collections) containing the same
// transaction repeatedly.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `txID` does not refer to a known transaction
func LookupCollectionByTransaction(r storage.Reader, txID flow.Identifier, collectionID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

// RemoveCollectionByTransactionIndex removes an entry in the index from transaction ID to collection containing the transaction.
// CAUTION: this is for recovery purposes only, and should not be used during normal operations!
// It returns nil if the collection does not exist.
// No errors are expected during normal operation.
func RemoveCollectionTransactionIndices(w storage.Writer, txID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexCollectionByTransaction, txID))
}

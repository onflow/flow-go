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

// UpsertCollection inserts a light collection into the storage, keyed by its ID.
//
// If the collection already exists, it will be overwritten. Note that here, the key (collection ID) is derived
// from the value (collection) via a collision-resistant hash function. Hence, unchecked overwrites pose no risk
// of data corruption, because for the same key, we expect the same value.
//
// No other errors are expected during normal operation.
func UpsertCollection(w storage.Writer, collection *flow.LightCollection) error {
	return UpsertByKey(w, MakePrefix(codeCollection, collection.ID()), collection)
}

// RetrieveCollection retrieves a collection by its ID. For efficiency, only a reduced representation is retrieved,
// where the constituent transactions are
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no collection with the specified `sealID` is known.
func RetrieveCollection(r storage.Reader, collID flow.Identifier, collection *flow.LightCollection) error {
	return RetrieveByKey(r, MakePrefix(codeCollection, collID), collection)
}

// RemoveCollection removes a collection from the storage.
// It returns nil if the collection does not exist.
// CAUTION: this is for recovery purposes only, and should not be used during normal operations
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
// No other errors are expected during normal operation.
func IndexCollectionPayload(lctx lockctx.Proof, w storage.Writer, clusterBlockID flow.Identifier, txIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}
	return UpsertByKey(w, MakePrefix(codeIndexCollection, clusterBlockID), txIDs)
}

// LookupCollectionPayload retrieves the list of transaction IDs that constitute the payload of the specified cluster block.
func LookupCollectionPayload(r storage.Reader, clusterBlockID flow.Identifier, txIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexCollection, clusterBlockID), txIDs)
}

// RemoveCollectionPayloadIndices removes a collection id indexed by a block id
// No errors are expected during normal operation.
func RemoveCollectionPayloadIndices(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexCollection, blockID))
}

// IndexCollectionByTransaction inserts a collection id keyed by a transaction id
// CAUTION with potentially OVERWRITING existing data:
// A transaction can belong to multiple collections, indexing a collection by a transaction
// will overwrite the previous collection id that was indexed by the same transaction id
// To prevent overwriting, check any existing value while holding storage.LockInsertCollection lock
func IndexCollectionByTransaction(lctx lockctx.Proof, w storage.Writer, txID flow.Identifier, collectionID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertCollection) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}

	return UpsertByKey(w, MakePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

// LookupCollectionByTransaction looks up the collection indexed by the given transaction ID,
// which is the collection in which the given transaction was included.
// It returns storage.ErrNotFound if the collection is not found.
// No errors are expected during normal operaion.
func LookupCollectionByTransaction(r storage.Reader, txID flow.Identifier, collectionID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

// RemoveCollectionByTransactionIndex removes a collection id indexed by a transaction id,
// created by [IndexCollectionByTransaction].
// No errors are expected during normal operation.
func RemoveCollectionTransactionIndices(w storage.Writer, txID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexCollectionByTransaction, txID))
}

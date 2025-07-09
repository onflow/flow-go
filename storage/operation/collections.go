package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// NOTE: These insert light collections, which only contain references
// to the constituent transactions. They do not modify transactions contained
// by the collections.

// UpsertCollection inserts a light collection into the storage.
// If the collection already exists, it will be overwritten.
func UpsertCollection(w storage.Writer, collection *flow.LightCollection) error {
	return UpsertByKey(w, MakePrefix(codeCollection, collection.ID()), collection)
}

func RetrieveCollection(r storage.Reader, collID flow.Identifier, collection *flow.LightCollection) error {
	return RetrieveByKey(r, MakePrefix(codeCollection, collID), collection)
}

// RemoveCollection removes a collection from the storage.
// It returns nil if the collection does not exist.
// No errors are expected during normal operation.
func RemoveCollection(w storage.Writer, collID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeCollection, collID))
}

// IndexCollectionPayload will overwrite any existing index, which is acceptable
// because the blockID is derived from txIDs within the payload, ensuring its uniqueness.
func IndexCollectionPayload(w storage.Writer, blockID flow.Identifier, txIDs []flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeIndexCollection, blockID), txIDs)
}

// LookupCollection looks up the collection for a given cluster payload.
func LookupCollectionPayload(r storage.Reader, blockID flow.Identifier, txIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexCollection, blockID), txIDs)
}

// RemoveCollectionPayloadIndices removes a collection id indexed by a block id
// No errors are expected during normal operation.
func RemoveCollectionPayloadIndices(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexCollection, blockID))
}

// UnsafeIndexCollectionByTransaction inserts a collection id keyed by a transaction id
// Unsafe because a transaction can belong to multiple collections, indexing collection by a transaction
// will overwrite the previous collection id that was indexed by the same transaction id
// To prevent overwritting, the caller must check if the transaction is already indexed, and make sure there
// is no dirty read before the writing by using locks.
func UnsafeIndexCollectionByTransaction(w storage.Writer, txID flow.Identifier, collectionID flow.Identifier) error {
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
// created by [UnsafeIndexCollectionByTransaction].
// No errors are expected during normal operation.
func RemoveCollectionTransactionIndices(w storage.Writer, txID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexCollectionByTransaction, txID))
}

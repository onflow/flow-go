package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// NOTE: These insert light collections, which only contain references
// to the constituent transactions. They do not modify transactions contained
// by the collections.

func InsertCollection(w storage.Writer, collection *flow.LightCollection) error {
	return UpsertByKey(w, MakePrefix(codeCollection, collection.ID()), collection)
}

func RetrieveCollection(r storage.Reader, collID flow.Identifier, collection *flow.LightCollection) error {
	return RetrieveByKey(r, MakePrefix(codeCollection, collID), collection)
}

func RemoveCollection(w storage.Writer, collID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeCollection, collID))
}

// IndexCollectionPayload indexes the transactions within the collection payload
// of a cluster block.
func IndexCollectionPayload(w storage.Writer, blockID flow.Identifier, txIDs []flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeIndexCollection, blockID), txIDs)
}

// LookupCollection looks up the collection for a given cluster payload.
func LookupCollectionPayload(r storage.Reader, blockID flow.Identifier, txIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexCollection, blockID), txIDs)
}

// UnsafeIndexCollectionByTransaction inserts a collection id keyed by a transaction id
// Unsafe because a transaction can belong to multiple collections, indexing collection by a transaction
// will overwrite the previous collection id that was indexed by the same transaction id
// To prevent overwritting, the caller must check if the transaction is already indexed, and make sure there
// is no dirty read before the writing by using locks.
func UnsafeIndexCollectionByTransaction(w storage.Writer, txID flow.Identifier, collectionID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

// LookupCollectionID retrieves a collection id by transaction id
func RetrieveCollectionID(r storage.Reader, txID flow.Identifier, collectionID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

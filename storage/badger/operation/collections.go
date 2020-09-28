// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// NOTE: These insert light collections, which only contain references
// to the constituent transactions. They do not modify transactions contained
// by the collections.

func InsertCollection(collection *flow.LightCollection) func(*badger.Txn) error {
	return insert(makePrefix(codeCollection, collection.ID()), collection)
}

func RetrieveCollection(collID flow.Identifier, collection *flow.LightCollection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollection, collID), collection)
}

func RemoveCollection(collID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeCollection, collID))
}

// IndexCollectionPayload indexes the transactions within the collection payload
// of a cluster block.
func IndexCollectionPayload(blockID flow.Identifier, txIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexCollection, blockID), txIDs)
}

// LookupCollection looks up the collection for a given cluster payload.
func LookupCollectionPayload(blockID flow.Identifier, txIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexCollection, blockID), txIDs)
}

// IndexCollectionByTransaction inserts a collection id keyed by a transaction id
func IndexCollectionByTransaction(txID flow.Identifier, collectionID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

// LookupCollectionID retrieves a collection id by transaction id
func RetrieveCollectionID(txID flow.Identifier, collectionID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

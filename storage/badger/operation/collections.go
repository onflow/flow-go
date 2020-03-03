// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// NOTE: These insert light collections, which only contain references
// to the constituent transactions. They do not modify transactions contained
// by the collections.

func InsertCollection(collection *flow.LightCollection) func(*badger.Txn) error {
	return insert(makePrefix(codeCollection, collection.ID()), collection)
}

func CheckCollection(collID flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeCollection, collID), exists)
}

// IndexCollection indexes the collection by payload hash.
func IndexCollection(payloadHash flow.Identifier, index uint64, collection *flow.LightCollection) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexCollection, payloadHash, index), collection.ID())
}

// LookupCollection looks up a collection ID by payload hash.
func LookupCollections(payloadHash flow.Identifier, collIDs *[]flow.Identifier) func(*badger.Txn) error {
	return traverse(makePrefix(codeIndexCollection, payloadHash), lookup(collIDs))
}

func RetrieveCollection(collID flow.Identifier, collection *flow.LightCollection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollection, collID), collection)
}

func RemoveCollection(collID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeCollection, collID))
}

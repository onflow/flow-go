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
	return insert(makePrefix(codeCollection, collection.Fingerprint()), collection)
}

func PersistCollection(collection *flow.LightCollection) func(*badger.Txn) error {
	return persist(makePrefix(codeCollection, collection.Fingerprint()), collection)
}

func RetrieveCollection(hash flow.Fingerprint, collection *flow.LightCollection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollection, hash), collection)
}

func RemoveCollection(hash flow.Fingerprint) func(*badger.Txn) error {
	return remove(makePrefix(codeCollection, hash))
}

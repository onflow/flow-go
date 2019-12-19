// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertNewGuaranteedCollections(hash crypto.Hash, collections []*collection.GuaranteedCollection) func(*badger.Txn) error {
	return insertNew(makePrefix(codeGuaranteedCollection, hash), collections)
}

func InsertGuaranteedCollections(hash crypto.Hash, collections []*collection.GuaranteedCollection) func(*badger.Txn) error {
	return insert(makePrefix(codeGuaranteedCollection, hash), collections)
}

func RetrieveGuaranteedCollections(hash crypto.Hash, collections *[]*collection.GuaranteedCollection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuaranteedCollection, hash), collections)
}

func InsertNewFlowCollection(collection *flow.Collection) func(*badger.Txn) error {
	return insertNew(makePrefix(codeCollection, collection.Hash()), collection)
}

func InsertFlowCollection(collection *flow.Collection) func(*badger.Txn) error {
	return insert(makePrefix(codeCollection, collection.Hash()), collection)
}

func RetrieveFlowCollection(hash crypto.Hash, collection *flow.Collection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollection, hash), collection)
}

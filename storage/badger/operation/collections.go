// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertGuaranteedCollections(hash crypto.Hash, collections []*flow.GuaranteedCollection) func(*badger.Txn) error {
	return insert(makePrefix(codeGuaranteedCollection, hash), collections)
}

func PersistGuaranteedCollections(hash crypto.Hash, collections []*flow.GuaranteedCollection) func(*badger.Txn) error {
	return persist(makePrefix(codeGuaranteedCollection, hash), collections)
}

func RetrieveGuaranteedCollections(hash crypto.Hash, collections *[]*flow.GuaranteedCollection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuaranteedCollection, hash), collections)
}

func InsertCollection(collection *flow.Collection) func(*badger.Txn) error {
	return insert(makePrefix(codeCollection, collection.Fingerprint()), collection)
}

func PersistCollection(collection *flow.Collection) func(*badger.Txn) error {
	return persist(makePrefix(codeCollection, collection.Fingerprint()), collection)
}

func RetrieveCollection(hash flow.Fingerprint, collection *flow.Collection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollection, hash), collection)
}

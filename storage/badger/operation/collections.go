// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertCollectionGuarantees(hash crypto.Hash, guarantees []*flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeGuaranteedCollection, hash), guarantees)
}

func PersistCollectionGuarantees(hash crypto.Hash, guarantees []*flow.CollectionGuarantee) func(*badger.Txn) error {
	return persist(makePrefix(codeGuaranteedCollection, hash), guarantees)
}

func RetrieveCollectionGuarantees(hash crypto.Hash, guarantees *[]*flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuaranteedCollection, hash), guarantees)
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

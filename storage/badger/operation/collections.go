// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertCollectionGuarantees(hash crypto.Hash, guarantees []*flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeCollections, hash), guarantees)
}

func PersistCollectionGuarantees(hash crypto.Hash, guarantees []*flow.CollectionGuarantee) func(*badger.Txn) error {
	return persist(makePrefix(codeCollections, hash), guarantees)
}

func RetrieveCollectionGuarantees(hash crypto.Hash, guarantees *[]*flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollections, hash), guarantees)
}

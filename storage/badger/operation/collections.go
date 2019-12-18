// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/storage"
)

func InsertNewCollections(hash crypto.Hash, collections []*collection.GuaranteedCollection) func(*badger.Txn) storage.Error {
	return insertNew(makePrefix(codeCollections, hash), collections)
}

func RetrieveCollections(hash crypto.Hash, collections *[]*collection.GuaranteedCollection) func(*badger.Txn) storage.Error {
	return retrieve(makePrefix(codeCollections, hash), collections)
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/storage"
)

func InsertNewHash(number uint64, hash crypto.Hash) func(*badger.Txn) storage.Error {
	return insertNew(makePrefix(codeHash, number), hash)
}

func RetrieveHash(number uint64, hash *crypto.Hash) func(*badger.Txn) storage.Error {
	return retrieve(makePrefix(codeHash, number), hash)
}

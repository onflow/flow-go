// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
)

func InsertHash(number uint64, hash crypto.Hash) func(*badger.Txn) error {
	return insert(makePrefix(codeHash, number), hash)
}

func RetrieveHash(number uint64, hash *crypto.Hash) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHash, number), hash)
}

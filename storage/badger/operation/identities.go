// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

func InsertNewIdentities(hash crypto.Hash, identities flow.IdentityList) func(*badger.Txn) storage.Error {
	return insertNew(makePrefix(codeIdentities, hash), identities)
}

func RetrieveIdentities(hash crypto.Hash, identities *flow.IdentityList) func(*badger.Txn) storage.Error {
	return retrieve(makePrefix(codeIdentities, hash), identities)
}

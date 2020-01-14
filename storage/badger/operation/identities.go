// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertIdentities(blockID flow.Identifier, identities flow.IdentityList) func(*badger.Txn) error {
	return insert(makePrefix(codeIdentities, blockID), identities)
}

func PersistIdentities(blockID flow.Identifier, identities flow.IdentityList) func(*badger.Txn) error {
	return persist(makePrefix(codeIdentities, blockID), identities)
}

func RetrieveIdentities(blockID flow.Identifier, identities *flow.IdentityList) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIdentities, blockID), identities)
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertIdentities(identities flow.IdentityList) func(*badger.Txn) error {
	return insert(makePrefix(codeIdentity), identities)
}

func RetrieveIdentities(identities *flow.IdentityList) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIdentity), identities)
}

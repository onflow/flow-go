// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

func InsertNewAddress(nodeID flow.Identifier, address string) func(*badger.Txn) storage.Error {
	return insertNew(makePrefix(codeAddress, nodeID), address)
}

func RetrieveAddress(nodeID flow.Identifier, address *string) func(*badger.Txn) storage.Error {
	return retrieve(makePrefix(codeAddress, nodeID), address)
}

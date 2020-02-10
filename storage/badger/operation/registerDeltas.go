package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertRegisterDelta(blockID flow.Identifier, delta *flow.RegisterDelta) func(*badger.Txn) error {
	return insert(makePrefix(codeRegisterDelta, blockID), delta)
}

func RetrieveRegisterDelta(blockID flow.Identifier, delta *flow.RegisterDelta) func(*badger.Txn) error {
	return retrieve(makePrefix(codeRegisterDelta, blockID), delta)
}

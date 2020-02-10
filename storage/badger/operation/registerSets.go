package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertRegisterSet(blockID flow.Identifier, set *flow.RegisterSet) func(*badger.Txn) error {
	return insert(makePrefix(codeRegisterSet, blockID), set)
}

func RetrieveRegisterSet(blockID flow.Identifier, set *flow.RegisterSet) func(*badger.Txn) error {
	return retrieve(makePrefix(codeRegisterSet, blockID), set)
}

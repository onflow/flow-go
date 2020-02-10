package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertRegisterSet(commit flow.StateCommitment, set *flow.RegisterSet) func(*badger.Txn) error {
	return insert(makePrefix(codeRegisterSet, commit), set)
}

func RetrieveRegisterSet(commit flow.StateCommitment, set *flow.RegisterSet) func(*badger.Txn) error {
	return retrieve(makePrefix(codeRegisterSet, commit), set)
}

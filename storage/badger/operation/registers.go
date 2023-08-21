package operation

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow"
)

// InsertRegister by the register path.
func InsertRegister(ID flow.RegisterID, entry *flow.RegisterEntry) func(*badger.Txn) error {
	return insert(makePrefix(codeRegister, ID), entry)
}

// RetrieveRegister by its path.
func RetrieveRegister(ID flow.RegisterID, entry *flow.RegisterEntry) func(*badger.Txn) error {
	return retrieve(makePrefix(codeRegister, ID), entry)
}

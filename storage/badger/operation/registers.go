package operation

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/ledger"
)

// InsertRegister by the register path.
func InsertRegister(path ledger.Path, payload *ledger.Payload) func(*badger.Txn) error {
	return insert(makePrefix(codeRegister, path), payload)
}

// RetrieveRegister by its path.
func RetrieveRegister(path ledger.Path, payload *ledger.Payload) func(*badger.Txn) error {
	return retrieve(makePrefix(codeRegister, path), payload)
}

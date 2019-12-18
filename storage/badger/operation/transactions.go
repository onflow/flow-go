package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertTransaction inserts a transaction keyed by transaction hash.
func InsertTransaction(hash crypto.Hash, tx *flow.Transaction) func(*badger.Txn) error {
	return insert(makePrefix(codeTransaction, hash), tx)
}

// RetrieveTransactions retrieves a transaction by hash.
func RetrieveTransaction(hash crypto.Hash, tx *flow.Transaction) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransaction, hash), tx)
}

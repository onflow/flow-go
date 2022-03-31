package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertTransaction inserts a transaction keyed by transaction fingerprint.
func InsertTransaction(txID flow.Identifier, tx *flow.TransactionBody) func(*badger.Txn) error {
	return insert(makePrefix(codeTransaction, txID), tx)
}

// RetrieveTransaction retrieves a transaction by fingerprint.
func RetrieveTransaction(txID flow.Identifier, tx *flow.TransactionBody) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransaction, txID), tx)
}

// HOTFIX: adding this to remove transactions without first checking existence.
func RemoveTransactionUnchecked(txID flow.Identifier) func(*badger.Txn) error {
	return removeUnchecked(makePrefix(codeTransaction, txID))
}

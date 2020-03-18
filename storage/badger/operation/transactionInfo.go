package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertTransactionInfo inserts a TransactionInfo keyed by transaction fingerprint.
func InsertTransactionInfo(tx *flow.TransactionInfo) func(*badger.Txn) error {
	return insert(makePrefix(codeTransactionInfo, tx.ID()), tx)
}

// RetrieveTransactionInfo retrieves a TransactionInfo by fingerprint.
func RetrieveTransactionInfo(txID flow.Identifier, tx *flow.TransactionInfo) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransactionInfo, txID), tx)
}

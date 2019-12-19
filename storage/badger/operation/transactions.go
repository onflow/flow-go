package operation

import (
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dgraph-io/badger/v2"
)

// InsertTransaction inserts a transaction keyed by transaction fingerprint.
func InsertTransaction(fingerprint model.Fingerprint, tx *flow.Transaction) func(*badger.Txn) error {
	return insert(makePrefix(codeTransaction, fingerprint), tx)
}

// RetrieveTransaction retrieves a transaction by fingerprint.
func RetrieveTransaction(fingerprint model.Fingerprint, tx *flow.Transaction) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransaction, fingerprint), tx)
}

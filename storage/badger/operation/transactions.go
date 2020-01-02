package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertTransaction inserts a transaction keyed by transaction fingerprint.
func InsertTransaction(fingerprint flow.Fingerprint, tx *flow.Transaction) func(*badger.Txn) error {
	return insert(makePrefix(codeTransaction, fingerprint), tx)
}

// RetrieveTransaction retrieves a transaction by fingerprint.
func RetrieveTransaction(fingerprint flow.Fingerprint, tx *flow.Transaction) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransaction, fingerprint), tx)
}

// RemoveTransaction removes the transaction with the given hash.
func RemoveTransaction(hash crypto.Hash) func(*badger.Txn) error {
	return remove(makePrefix(codeTransaction, hash))
}

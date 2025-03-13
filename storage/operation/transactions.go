package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpsertTransaction inserts a transaction keyed by transaction fingerprint.
// It overwrites any existing transaction, which is ok because tx is unique by its ID
func UpsertTransaction(w storage.Writer, txID flow.Identifier, tx *flow.TransactionBody) error {
	return UpsertByKey(w, MakePrefix(codeTransaction, txID), tx)
}

// RetrieveTransaction retrieves a transaction by fingerprint.
func RetrieveTransaction(r storage.Reader, txID flow.Identifier, tx *flow.TransactionBody) error {
	return RetrieveByKey(r, MakePrefix(codeTransaction, txID), tx)
}

// RemoveTransaction removes a transaction by fingerprint.
func RemoveTransaction(r storage.Writer, txID flow.Identifier) error {
	return RemoveByKey(r, MakePrefix(codeTransaction, txID))
}

package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertTransaction inserts a transaction keyed by transaction fingerprint.
func InsertTransaction(w storage.Writer, txID flow.Identifier, tx *flow.TransactionBody) error {
	return UpsertByKey(w, MakePrefix(codeTransaction, txID), tx)
}

// RetrieveTransaction retrieves a transaction by fingerprint.
func RetrieveTransaction(r storage.Reader, txID flow.Identifier, tx *flow.TransactionBody) error {
	return RetrieveByKey(r, MakePrefix(codeTransaction, txID), tx)
}

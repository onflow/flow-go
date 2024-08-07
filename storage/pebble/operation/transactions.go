package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
)

// InsertTransaction inserts a transaction keyed by transaction fingerprint.
func InsertTransaction(txID flow.Identifier, tx *flow.TransactionBody) func(pebble.Writer) error {
	return insert(makePrefix(codeTransaction, txID), tx)
}

// RetrieveTransaction retrieves a transaction by fingerprint.
func RetrieveTransaction(txID flow.Identifier, tx *flow.TransactionBody) func(pebble.Reader) error {
	return retrieve(makePrefix(codeTransaction, txID), tx)
}

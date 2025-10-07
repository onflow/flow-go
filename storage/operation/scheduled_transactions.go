package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RetrieveTransactionIDByScheduledTransactionID retrieves the transaction ID of the scheduled
// transaction by its scheduled transaction ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no transaction ID is found for the given scheduled transaction ID
func RetrieveTransactionIDByScheduledTransactionID(r storage.Reader, scheduledTxID uint64, txID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeTransactionIDByScheduledTransactionID, scheduledTxID), txID)
}

// RetrieveBlockIDByScheduledTransactionID retrieves the block ID of the scheduled transaction by its transaction ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no block ID is found for the given transaction ID
func RetrieveBlockIDByScheduledTransactionID(r storage.Reader, txID flow.Identifier, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDByScheduledTransactionID, txID), blockID)
}

// BatchIndexScheduledTransactionID indexes the scheduled transaction by its scheduled transaction ID and transaction ID.
//
// No errors are expected during normal operation.
func BatchIndexScheduledTransactionID(w storage.Writer, scheduledTxID uint64, txID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeTransactionIDByScheduledTransactionID, scheduledTxID), scheduledTxID)
}

// BatchIndexScheduledTransactionBlockID indexes the scheduled transaction by its transaction ID and block ID.
//
// No errors are expected during normal operation.
func BatchIndexScheduledTransactionBlockID(w storage.Writer, txID flow.Identifier, blockID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeBlockIDByScheduledTransactionID, txID), blockID)
}

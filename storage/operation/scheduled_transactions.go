package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

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

// IndexScheduledTransactionID indexes the scheduled transaction's transaction ID by its scheduled transaction ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if the scheduled transaction ID is already indexed
func IndexScheduledTransactionID(lctx lockctx.Proof, rw storage.ReaderBatchWriter, scheduledTxID uint64, txID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockIndexScheduledTransaction) {
		return fmt.Errorf("missing lock: %v", storage.LockIndexScheduledTransaction)
	}

	key := MakePrefix(codeTransactionIDByScheduledTransactionID, scheduledTxID)
	exists, err := KeyExists(rw.GlobalReader(), key)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("scheduled transaction ID already indexed: %w", storage.ErrAlreadyExists)
	}

	return UpsertByKey(rw.Writer(), key, txID)
}

// IndexScheduledTransactionBlockID indexes the scheduled transaction's block ID by its transaction ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if the scheduled transaction block ID is already indexed
func IndexScheduledTransactionBlockID(lctx lockctx.Proof, rw storage.ReaderBatchWriter, txID flow.Identifier, blockID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockIndexScheduledTransaction) {
		return fmt.Errorf("missing lock: %v", storage.LockIndexScheduledTransaction)
	}

	key := MakePrefix(codeBlockIDByScheduledTransactionID, txID)
	exists, err := KeyExists(rw.GlobalReader(), key)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("scheduled transaction block ID already indexed: %w", storage.ErrAlreadyExists)
	}

	return UpsertByKey(rw.Writer(), key, blockID)
}

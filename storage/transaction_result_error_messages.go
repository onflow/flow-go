package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionResultErrorMessagesReader represents persistent storage read operations for transaction result error messages
type TransactionResultErrorMessagesReader interface {
	// Exists returns true if transaction result error messages for the given ID have been stored.
	//
	// Note that transaction error messages are auxiliary data provided by the Execution Nodes on a goodwill basis and
	// not protected by the protocol. Execution Error messages might be non-deterministic, i.e. potentially different
	// for different execution nodes.
	//
	// No errors are expected during normal operation.
	Exists(blockID flow.Identifier) (bool, error)

	// ByBlockIDTransactionID returns the transaction result error message for the given block ID and transaction ID.
	//
	// Note that transaction error messages are auxiliary data provided by the Execution Nodes on a goodwill basis and
	// not protected by the protocol. Execution Error messages might be non-deterministic, i.e. potentially different
	// for different execution nodes.
	//
	// Expected errors during normal operation:
	//   - [storage.ErrNotFound] if no transaction error message is known at given block and transaction id.
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResultErrorMessage, error)

	// ByBlockIDTransactionIndex returns the transaction result error message for the given blockID and transaction index.
	//
	// Note that transaction error messages are auxiliary data provided by the Execution Nodes on a goodwill basis and
	// not protected by the protocol. Execution Error messages might be non-deterministic, i.e. potentially different
	// for different execution nodes.
	//
	// Expected errors during normal operation:
	//   - [storage.ErrNotFound] if no transaction error message is known at given block and transaction index.
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResultErrorMessage, error)

	// ByBlockID gets all transaction result error messages for a block, ordered by transaction index.
	// CAUTION: This method will return an empty slice both if the block is not indexed yet and if the block does not have any errors.
	//
	// Note that transaction error messages are auxiliary data provided by the Execution Nodes on a goodwill basis and
	// not protected by the protocol. Execution Error messages might be non-deterministic, i.e. potentially different
	// for different execution nodes.
	//
	// No errors are expected during normal operations.
	ByBlockID(id flow.Identifier) ([]flow.TransactionResultErrorMessage, error)
}

// TransactionResultErrorMessages represents persistent storage for transaction result error messages
type TransactionResultErrorMessages interface {
	TransactionResultErrorMessagesReader

	// Store persists and indexes all transaction result error messages for the given blockID. The caller must
	// acquire [storage.LockInsertTransactionResultErrMessage] and hold it until the write batch has been committed.
	// It returns [storage.ErrAlreadyExists] if tx result error messages for the block already exist.
	Store(lctx lockctx.Proof, blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage) error

	// BatchStore persists and indexes all transaction result error messages for the given blockID as part
	// of the provided batch. The caller must acquire [storage.LockInsertTransactionResultErrMessage] and
	// hold it until the write batch has been committed.
	// It returns [storage.ErrAlreadyExists] if tx result error messages for the block already exist.
	BatchStore(lctx lockctx.Proof, rw ReaderBatchWriter, blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage) error
}

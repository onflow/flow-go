package inmemory

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type TransactionResultErrorMessagesReader struct {
	blockID     flow.Identifier
	errMessages []flow.TransactionResultErrorMessage
	byTxID      map[flow.Identifier]*flow.TransactionResultErrorMessage
	byTxIndex   map[uint32]*flow.TransactionResultErrorMessage
}

var _ storage.TransactionResultErrorMessagesReader = (*TransactionResultErrorMessagesReader)(nil)

func NewTransactionResultErrorMessages(blockID flow.Identifier, errMessages []flow.TransactionResultErrorMessage) *TransactionResultErrorMessagesReader {
	byTxID := make(map[flow.Identifier]*flow.TransactionResultErrorMessage)
	byTxIndex := make(map[uint32]*flow.TransactionResultErrorMessage)

	for i, errMessage := range errMessages {
		byTxID[errMessage.TransactionID] = &errMessage
		byTxIndex[uint32(i)] = &errMessage
	}

	return &TransactionResultErrorMessagesReader{
		blockID:     blockID,
		errMessages: errMessages,
		byTxID:      byTxID,
		byTxIndex:   byTxIndex,
	}
}

// Exists returns true if transaction result error messages for the given ID have been stored.
//
// No error returns are expected during normal operation.
func (t *TransactionResultErrorMessagesReader) Exists(blockID flow.Identifier) (bool, error) {
	_, err := t.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// ByBlockIDTransactionID returns the transaction result error message for the given block ID and transaction ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no transaction error message is known at given block and transaction id.
func (t *TransactionResultErrorMessagesReader) ByBlockIDTransactionID(
	blockID flow.Identifier,
	transactionID flow.Identifier,
) (*flow.TransactionResultErrorMessage, error) {
	if t.blockID != blockID {
		return nil, storage.ErrNotFound
	}

	val, ok := t.byTxID[transactionID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// ByBlockIDTransactionIndex returns the transaction result error message for the given blockID and transaction index.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no transaction error message is known at given block and transaction index.
func (t *TransactionResultErrorMessagesReader) ByBlockIDTransactionIndex(
	blockID flow.Identifier,
	txIndex uint32,
) (*flow.TransactionResultErrorMessage, error) {
	if t.blockID != blockID {
		return nil, storage.ErrNotFound
	}

	val, ok := t.byTxIndex[txIndex]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// ByBlockID gets all transaction result error messages for a block, ordered by transaction index.
// Note: This method will return an empty slice both if the block is not indexed yet and if the block does not have any errors.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no block was found.
func (t *TransactionResultErrorMessagesReader) ByBlockID(id flow.Identifier) ([]flow.TransactionResultErrorMessage, error) {
	if t.blockID != id {
		return nil, storage.ErrNotFound
	}

	return t.errMessages, nil
}

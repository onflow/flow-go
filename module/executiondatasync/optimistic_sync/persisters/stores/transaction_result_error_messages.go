package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ PersisterStore = (*TxResultErrMsgStore)(nil)

// TxResultErrMsgStore handles persisting transaction result error messages
type TxResultErrMsgStore struct {
	data           []flow.TransactionResultErrorMessage
	txResultErrMsg storage.TransactionResultErrorMessages
	blockID        flow.Identifier
	lockManager    storage.LockManager
}

func NewTxResultErrMsgStore(
	data []flow.TransactionResultErrorMessage,
	txResultErrMsg storage.TransactionResultErrorMessages,
	blockID flow.Identifier,
	lockManager storage.LockManager,
) *TxResultErrMsgStore {
	return &TxResultErrMsgStore{
		data:           data,
		txResultErrMsg: txResultErrMsg,
		blockID:        blockID,
		lockManager:    lockManager,
	}
}

// Persist saves and indexes all transaction result error messages for the block as part of the
// provided database batch. The caller must acquire [storage.LockInsertTransactionResultErrMessage]
// and hold it until the write batch has been committed.
// Will return an error if the transaction result error messages for the block already exist.
//
// No error returns are expected during normal operations
func (t *TxResultErrMsgStore) Persist(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
	err := t.txResultErrMsg.BatchStore(lctx, rw, t.blockID, t.data)
	if err != nil {
		return fmt.Errorf("could not add transaction result error messages to batch: %w", err)
	}
	return nil
}

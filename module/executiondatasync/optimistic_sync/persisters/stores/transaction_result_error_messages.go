package stores

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ PersisterStore = (*TxResultErrMsgStore)(nil)

// TxResultErrMsgStore handles persisting transaction result error messages
type TxResultErrMsgStore struct {
	data                    []flow.TransactionResultErrorMessage
	persistedTxResultErrMsg storage.TransactionResultErrorMessages
	blockID                 flow.Identifier
	lockManager             storage.LockManager
}

func NewTxResultErrMsgStore(
	data []flow.TransactionResultErrorMessage,
	persistedTxResultErrMsg storage.TransactionResultErrorMessages,
	blockID flow.Identifier,
	lockManager storage.LockManager,
) *TxResultErrMsgStore {
	return &TxResultErrMsgStore{
		data:                    data,
		persistedTxResultErrMsg: persistedTxResultErrMsg,
		blockID:                 blockID,
		lockManager:             lockManager,
	}
}

// Persist adds transaction result error messages to the batch.
//
// No error returns are expected during normal operations
func (t *TxResultErrMsgStore) Persist(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
	err := t.persistedTxResultErrMsg.BatchStore(lctx, rw, t.blockID, t.data)
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			return nil
		}
		return fmt.Errorf("could not add transaction result error messages to batch: %w", err)
	}
	return nil
}

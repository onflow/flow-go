package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

var _ PersisterStore = (*TxResultErrMsgStore)(nil)

// TxResultErrMsgStore handles persisting transaction result error messages
type TxResultErrMsgStore struct {
	inMemoryTxResultErrMsg  *unsynchronized.TransactionResultErrorMessages
	persistedTxResultErrMsg storage.TransactionResultErrorMessages
	blockID                 flow.Identifier
	lockManager             storage.LockManager
}

func NewTxResultErrMsgStore(
	inMemoryTxResultErrMsg *unsynchronized.TransactionResultErrorMessages,
	persistedTxResultErrMsg storage.TransactionResultErrorMessages,
	blockID flow.Identifier,
	lockManager storage.LockManager,
) *TxResultErrMsgStore {
	return &TxResultErrMsgStore{
		inMemoryTxResultErrMsg:  inMemoryTxResultErrMsg,
		persistedTxResultErrMsg: persistedTxResultErrMsg,
		blockID:                 blockID,
		lockManager:             lockManager,
	}
}

// Persist adds transaction result error messages to the batch.
// No errors are expected during normal operations
func (t *TxResultErrMsgStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	txResultErrMsgs, err := t.inMemoryTxResultErrMsg.ByBlockID(t.blockID)
	if err != nil {
		return fmt.Errorf("could not get transaction result error messages: %w", err)
	}

	if len(txResultErrMsgs) > 0 {
		// Use storage.WithLock to acquire the necessary lock and store the error messages
		err := storage.WithLock(t.lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
			return t.persistedTxResultErrMsg.BatchStore(lctx, batch, t.blockID, txResultErrMsgs)
		})
		if err != nil {
			return fmt.Errorf("could not add transaction result error messages to batch: %w", err)
		}
	}

	return nil
}

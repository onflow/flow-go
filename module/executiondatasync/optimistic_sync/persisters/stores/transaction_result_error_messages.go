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
	data                    []flow.TransactionResultErrorMessage
	persistedTxResultErrMsg storage.TransactionResultErrorMessages
	blockID                 flow.Identifier
}

func NewTxResultErrMsgStore(
	data []flow.TransactionResultErrorMessage,
	persistedTxResultErrMsg storage.TransactionResultErrorMessages,
	blockID flow.Identifier,
) *TxResultErrMsgStore {
	return &TxResultErrMsgStore{
		data:                    data,
		persistedTxResultErrMsg: persistedTxResultErrMsg,
		blockID:                 blockID,
	}
}

// Persist adds transaction result error messages to the batch.
//
// No error returns are expected during normal operations
func (t *TxResultErrMsgStore) Persist(_ lockctx.Proof, batch storage.ReaderBatchWriter) error {
	if len(t.data) > 0 {
		if err := t.persistedTxResultErrMsg.BatchStore(t.blockID, t.data, batch); err != nil {
			return fmt.Errorf("could not add transaction result error messages to batch: %w", err)
		}
	}

	return nil
}

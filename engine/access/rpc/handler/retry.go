package handler

import (
	"context"
	"sync"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
)

// retryFrequency has to be less than TransactionExpiry or else this module does nothing
const retryFrequency uint64 = 120 // blocks

// Retry implements a simple retrier,
type Retry struct {
	mu sync.RWMutex
	// pending transactions
	transactionByReferencBlockHeight map[uint64]map[flow.Identifier]*flow.TransactionBody
	txHandler                        *Handler
}

func newRetry() *Retry {
	return &Retry{
		transactionByReferencBlockHeight: map[uint64]map[flow.Identifier]*flow.TransactionBody{},
	}
}

func (r *Retry) SetHandler(rpc *Handler) *Retry {
	r.txHandler = rpc
	return r
}

func (r *Retry) Retry(height uint64) {
	// No need to retry if height is lower than DefaultTransactionExpiry
	if height < flow.DefaultTransactionExpiry {
		return
	}

	// naive cleanup for now, prune every 120 blocks
	if height%retryFrequency == 0 {
		r.prune(height)
	}

	heightToRetry := height - flow.DefaultTransactionExpiry + retryFrequency

	for heightToRetry < height {
		r.retryTxsAtHeight(heightToRetry)

		heightToRetry = heightToRetry + retryFrequency
	}

}

func (b *Retry) Notify(signal interface{}) bool {
	height, ok := signal.(uint64)
	if !ok {
		return false
	}
	b.Retry(height)
	return true
}

// RegisterTransaction adds a transaction that could possibly be retried
func (r *Retry) RegisterTransaction(height uint64, tx *flow.TransactionBody) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.transactionByReferencBlockHeight[height] == nil {
		r.transactionByReferencBlockHeight[height] = make(map[flow.Identifier]*flow.TransactionBody)
	}
	r.transactionByReferencBlockHeight[height][tx.ID()] = tx
}

func (r *Retry) prune(height uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// If height is less than the default, there will be no expired transactions
	if height < flow.DefaultTransactionExpiry {
		return
	}
	for h := range r.transactionByReferencBlockHeight {
		if h < height-flow.DefaultTransactionExpiry {
			delete(r.transactionByReferencBlockHeight, h)
		}
	}
}

func (r *Retry) retryTxsAtHeight(heightToRetry uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	txsAtHeight := r.transactionByReferencBlockHeight[heightToRetry]
	for txID, tx := range txsAtHeight {
		status, err := r.txHandler.DeriveTransactionStatus(tx, false)
		if err != nil {
			continue
		}
		if status == entities.TransactionStatus_PENDING {
			txMsg := convert.TransactionToMessage(*tx)
			_, _ = r.txHandler.SendRawTransaction(context.Background(), txMsg)
		} else if status != entities.TransactionStatus_UNKNOWN {
			// not pending or unknown, don't need to retry anymore
			delete(txsAtHeight, txID)
		}
	}
}

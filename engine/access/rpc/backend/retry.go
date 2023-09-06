package backend

import (
	"context"
	"errors"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// retryFrequency has to be less than TransactionExpiry or else this module does nothing
const retryFrequency uint64 = 120 // Blocks

// Retry implements a simple retry mechanism for transaction submission.
type Retry struct {
	mu sync.RWMutex
	// pending Transactions
	transactionByReferencBlockHeight map[uint64]map[flow.Identifier]*flow.TransactionBody
	backend                          *Backend
	active                           bool
}

func newRetry() *Retry {
	return &Retry{
		transactionByReferencBlockHeight: map[uint64]map[flow.Identifier]*flow.TransactionBody{},
	}
}

func (r *Retry) Activate() *Retry {
	r.active = true
	return r
}

func (r *Retry) IsActive() bool {
	return r.active
}

func (r *Retry) SetBackend(b *Backend) *Retry {
	r.backend = b
	return r
}

func (r *Retry) Retry(height uint64) {
	// No need to retry if height is lower than DefaultTransactionExpiry
	if height < flow.DefaultTransactionExpiry {
		return
	}

	// naive cleanup for now, prune every 120 Blocks
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
	// If height is less than the default, there will be no expired Transactions
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
		// find the block for the transaction
		block, err := r.backend.lookupBlock(txID)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				continue
			}
			block = nil
		}

		// find the transaction status
		status, err := r.backend.deriveTransactionStatus(tx, false, block)
		if err != nil {
			continue
		}
		if status == flow.TransactionStatusPending {
			_ = r.backend.SendRawTransaction(context.Background(), tx)
		} else if status != flow.TransactionStatusUnknown {
			// not pending or unknown, don't need to retry anymore
			delete(txsAtHeight, txID)
		}
	}
}

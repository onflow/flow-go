package backend

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state"
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
	log                              zerolog.Logger // default logger
}

func newRetry(log zerolog.Logger) *Retry {
	return &Retry{
		log:                              log,
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

// Retry attempts to resend transactions for a specified block height.
// It performs cleanup operations, including pruning old transactions, and retries sending
// transactions that are still pending.
// The method takes a block height as input. If the provided height is lower than
// flow.DefaultTransactionExpiry, no retries are performed, and the method returns nil.
// No errors expected during normal operations.
func (r *Retry) Retry(height uint64) error {
	// No need to retry if height is lower than DefaultTransactionExpiry
	if height < flow.DefaultTransactionExpiry {
		return nil
	}

	// naive cleanup for now, prune every 120 Blocks
	if height%retryFrequency == 0 {
		r.prune(height)
	}

	heightToRetry := height - flow.DefaultTransactionExpiry + retryFrequency

	for heightToRetry < height {
		err := r.retryTxsAtHeight(heightToRetry)
		if err != nil {
			return err
		}
		heightToRetry = heightToRetry + retryFrequency
	}
	return nil
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

// retryTxsAtHeight retries transactions at a specific block height.
// It looks up transactions at the specified height and retries sending
// raw transactions for those that are still pending. It also cleans up
// transactions that are no longer pending or have an unknown status.
// Error returns:
//   - errors are unexpected and potentially symptoms of internal implementation bugs or state corruption (fatal).
func (r *Retry) retryTxsAtHeight(heightToRetry uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	txsAtHeight := r.transactionByReferencBlockHeight[heightToRetry]
	for txID, tx := range txsAtHeight {
		// find the block for the transaction
		block, err := r.backend.lookupBlock(txID)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return err
			}
			block = nil
		}

		// find the transaction status
		status, err := r.backend.DeriveTransactionStatus(tx, false, block)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				return err
			}
			continue
		}
		if status == flow.TransactionStatusPending {
			err = r.backend.SendRawTransaction(context.Background(), tx)
			if err != nil {
				r.log.Info().Str("retry", fmt.Sprintf("retryTxsAtHeight: %v", heightToRetry)).Err(err).Msg("failed to send raw transactions")
			}
		} else if status != flow.TransactionStatusUnknown {
			// not pending or unknown, don't need to retry anymore
			delete(txsAtHeight, txID)
		}
	}
	return nil
}

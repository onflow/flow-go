package retrier

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RetryFrequency has to be less than TransactionExpiry or else this module does nothing
const RetryFrequency uint64 = 120 // Blocks

type Transactions map[flow.Identifier]*flow.TransactionBody
type BlockHeightToTransactions map[uint64]Transactions

type TransactionSender interface {
	SendRawTransaction(ctx context.Context, tx *flow.TransactionBody) error
}

type Retrier interface {
	Retry(height uint64) error
	RegisterTransaction(height uint64, tx *flow.TransactionBody)
}

// RetrierImpl implements a simple retry mechanism for transaction submission.
type RetrierImpl struct {
	log zerolog.Logger

	mu                  sync.RWMutex
	pendingTransactions BlockHeightToTransactions

	blocks      storage.Blocks
	collections storage.Collections

	txSender        TransactionSender
	txStatusDeriver *status.TxStatusDeriver
}

func NewRetrier(
	log zerolog.Logger,
	blocks storage.Blocks,
	collections storage.Collections,
	txSender TransactionSender,
	txStatusDeriver *status.TxStatusDeriver,
) *RetrierImpl {
	return &RetrierImpl{
		log:                 log,
		pendingTransactions: BlockHeightToTransactions{},
		blocks:              blocks,
		collections:         collections,
		txSender:            txSender,
		txStatusDeriver:     txStatusDeriver,
	}
}

// Retry attempts to resend transactions for a specified block height.
// It performs cleanup operations, including pruning old transactions, and retries sending
// transactions that are still pending.
// The method takes a block height as input. If the provided height is lower than
// flow.DefaultTransactionExpiry, no retries are performed, and the method returns nil.
// No errors expected during normal operations.
func (r *RetrierImpl) Retry(height uint64) error {
	// No need to retry if height is lower than DefaultTransactionExpiry
	if height < flow.DefaultTransactionExpiry {
		return nil
	}

	// naive cleanup for now, prune every 120 Blocks
	if height%RetryFrequency == 0 {
		r.prune(height)
	}

	heightToRetry := height - flow.DefaultTransactionExpiry + RetryFrequency

	for heightToRetry < height {
		err := r.retryTxsAtHeight(heightToRetry)
		if err != nil {
			return err
		}
		heightToRetry = heightToRetry + RetryFrequency
	}
	return nil
}

// RegisterTransaction adds a transaction that could possibly be retried
func (r *RetrierImpl) RegisterTransaction(height uint64, tx *flow.TransactionBody) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pendingTransactions[height] == nil {
		r.pendingTransactions[height] = make(map[flow.Identifier]*flow.TransactionBody)
	}
	r.pendingTransactions[height][tx.ID()] = tx
}

func (r *RetrierImpl) prune(height uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// If height is less than the default, there will be no expired Transactions
	if height < flow.DefaultTransactionExpiry {
		return
	}
	for h := range r.pendingTransactions {
		if h < height-flow.DefaultTransactionExpiry {
			delete(r.pendingTransactions, h)
		}
	}
}

// retryTxsAtHeight retries transactions at a specific block height.
// It looks up transactions at the specified height and retries sending
// raw transactions for those that are still pending. It also cleans up
// transactions that are no longer pending or have an unknown status.
//
// No errors expected during normal operations.
func (r *RetrierImpl) retryTxsAtHeight(heightToRetry uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	txsAtHeight := r.pendingTransactions[heightToRetry]
	for txID, tx := range txsAtHeight {
		// find the block for the transaction
		block, err := r.lookupBlock(txID)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return err
			}
			block = nil
		}

		// find the transaction status
		var status flow.TransactionStatus
		if block == nil {
			status, err = r.txStatusDeriver.DeriveUnknownTransactionStatus(tx.ReferenceBlockID)
		} else {
			status, err = r.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, false)
		}
		if err != nil {
			return fmt.Errorf("failed to derive transaction status: %w", err)
		}

		if status == flow.TransactionStatusPending {
			err = r.txSender.SendRawTransaction(context.Background(), tx)
			if err != nil {
				r.log.Info().
					Str("retry", fmt.Sprintf("retryTxsAtHeight: %v", heightToRetry)).
					Err(err).
					Msg("failed to send raw transactions")
			}
		} else if status != flow.TransactionStatusUnknown {
			// not pending or unknown, don't need to retry anymore
			delete(txsAtHeight, txID)
		}
	}
	return nil
}

// Error returns:
//   - `storage.ErrNotFound` - collection referenced by transaction or block by a collection has not been found.
//   - all other errors are unexpected and potentially symptoms of internal implementation bugs or state corruption (fatal).
func (r *RetrierImpl) lookupBlock(txID flow.Identifier) (*flow.Block, error) {
	collection, err := r.collections.LightByTransactionID(txID)
	if err != nil {
		return nil, err
	}

	block, err := r.blocks.ByCollectionID(collection.ID())
	if err != nil {
		return nil, err
	}

	return block, nil
}

package matching

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/utils/logging"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Indexer subscribe the block finalization event, and index the receipts in the
// block payload by the executed block id.
type Indexer struct {
	notifications.NoopConsumer

	log        zerolog.Logger
	receiptsDB storage.ExecutionReceipts
	payloadsDB storage.Payloads
}

func NewIndexer(log zerolog.Logger, receiptsDB storage.ExecutionReceipts, payloadsDB storage.Payloads) *Indexer {
	return &Indexer{
		log:        log.With().Str("engine", "matching.Indexer").Logger(),
		receiptsDB: receiptsDB,
		payloadsDB: payloadsDB,
	}
}

// OnBlockIncorporated implements the callback, which HotStuff executes when a block is incorporated.
func (i *Indexer) OnBlockIncorporated(block *model.Block) {
	payload, err := i.payloadsDB.ByBlockID(block.BlockID)
	if err != nil {
		i.log.Fatal().Err(err).
			Hex("block_id", logging.ID(block.BlockID)).
			Msg("could not get payload for block")
	}

	for _, receipt := range payload.Receipts {
		err := i.indexReceipt(receipt)
		if err != nil {
			i.log.Fatal().Err(err).
				Hex("receipt_id", logging.Entity(receipt)).
				Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
				Msg("internal error indexing receipts in incorporated block")
		}
	}
}

// indexReceipts populates the index:
//     executed blockID -> ExecutorID -> ReceiptID
// CAUTION: temporary solution
// * We currently rely on this index to determine whether we have seen
//   consistent receipts from _at least two_ Execution Nodes.
//   Hence, this logic is critical for sealing liveness.
// * Therefore, we must guarantee that receipts from honest executors
//   _eventually_ end up in the index. We consider executors as honest,
//   if and only if they produce a single receipt for each block
//   (Caution: this is disregarding some protocol edge cases of the
//   mature protocol; but we don't have this edge cases supported
//   anyway at the moment, so there is no problem)
// Error return:
// * all returned errors are unexpected internal errors, which should be fatal
func (i *Indexer) indexReceipt(receipt *flow.ExecutionReceipt) error {
	err := i.receiptsDB.IndexByExecutor(receipt)
	if err != nil {
		if errors.Is(err, storage.ErrDataMismatch) {
			// This happens if the EN produces different receipts.
			// In this case, we consider the EN as dishonest. We only index the first
			// receipt and do _not_ add subsequent receipts from the same EN to the index.
			// This should not be a liveness problem, as we TEMPORARILY assume
			// that there are at least two honest ENs.
			// TODO: update as part of https://github.com/dapperlabs/flow-go/issues/5326
			i.log.Warn().Err(err).
				Hex("executor", logging.ID(receipt.ExecutorID)).
				Msg("execution node produced different receipts for the same block")
		}
		return fmt.Errorf("internal error indexing receipt by executor: %w", err)
	}
	return nil
}

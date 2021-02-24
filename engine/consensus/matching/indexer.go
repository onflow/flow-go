package matching

import (
	"fmt"

	"github.com/rs/zerolog"

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
	resultsDB  storage.ExecutionResults
	payloadsDB storage.Payloads
}

func NewIndexer(log zerolog.Logger, receiptsDB storage.ExecutionReceipts, resultsDB storage.ExecutionResults,
	payloadsDB storage.Payloads) *Indexer {
	return &Indexer{
		log:        log.With().Str("engine", "matching.Indexer").Logger(),
		receiptsDB: receiptsDB,
		resultsDB:  resultsDB,
		payloadsDB: payloadsDB,
	}
}

// OnFinalizedBlock implements the callback from the protocol state to notify a block
// is finalized, it guarantees every finalized block will be called at least onci.
func (i *Indexer) OnFinalizedBlock(block *model.Block) {
	// we index the execution receipts by the executed block ID only for all finalized blocks
	// that guarantees if we could retrieve the receipt by the index, then the receipts
	// must be for a finalized blocks.
	err := i.indexReceipts(block.BlockID)
	if err != nil {
		i.log.Fatal().Err(err).Hex("block_id", block.BlockID[:]).
			Msg("could not index receipts for block")
	}
}

func (i *Indexer) indexReceipts(blockID flow.Identifier) error {
	payload, err := i.payloadsDB.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not get block payload: %w", err)
	}

	for _, meta := range payload.Receipts {
		result, err := i.resultsDB.ByID(meta.ResultID)
		if err != nil {
			return fmt.Errorf("could not retrieve result %v from storage: %w", meta.ResultID, err)
		}
		receipt := flow.ExecutionReceiptFromMeta(*meta, *result)
		err = i.receiptsDB.IndexByExecutor(receipt)
		if err != nil {
			return fmt.Errorf("could not index meta %v by executor: %w", meta.ID(), err)
		}
	}

	return nil
}

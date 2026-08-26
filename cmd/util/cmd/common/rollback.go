package common

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// RemoveExecutionResultsFromHeight removes all execution results and related data for
// every block at or above fromHeight, including both finalized and pending blocks.
// It returns the set of chunk IDs that were removed from the protocol-state DB so
// that the caller can delete the corresponding chunk-data-packs from the chunk DB.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if a required block header or execution result is absent
func RemoveExecutionResultsFromHeight(
	protocolDBBatch storage.Batch,
	protoState protocol.State,
	transactionResults storage.TransactionResults,
	commits storage.Commits,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	myReceipts storage.MyExecutionReceipts,
	events storage.Events,
	serviceEvents storage.ServiceEvents,
	fromHeight uint64,
) ([]flow.Identifier, error) {
	log.Info().Msgf("removing results for blocks from height: %v", fromHeight)

	root := protoState.Params().FinalizedRoot()

	if fromHeight <= root.Height {
		return nil, fmt.Errorf("can only remove results for blocks above root: fromHeight %v, rootHeight %v",
			fromHeight, root.Height)
	}

	final, err := protoState.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized head: %w", err)
	}

	if fromHeight > final.Height {
		return nil, fmt.Errorf("cannot remove results for unfinalized height %v (finalized: %v)",
			fromHeight, final.Height)
	}

	var allChunkIDs []flow.Identifier

	pendings, err := protoState.Final().Descendants()
	if err != nil {
		return nil, fmt.Errorf("could not get pending descendants: %w", err)
	}

	// Remove pending descendants before finalized blocks, and iterate in reverse so
	// that deeper descendants are removed before their ancestors, avoiding gaps if
	// the operation is interrupted.
	for i := len(pendings) - 1; i >= 0; i-- {
		pending := pendings[i]
		chunkIDs, err := RemoveExecutionResultsForBlock(
			protocolDBBatch, commits, transactionResults, results,
			chunkDataPacks, myReceipts, events, serviceEvents, pending)
		if err != nil {
			return nil, fmt.Errorf("could not remove result for pending block %v: %w", pending, err)
		}

		allChunkIDs = append(allChunkIDs, chunkIDs...)
		log.Info().Msgf("removed result for pending block %v (%v/%v)", pending, len(pendings)-i, len(pendings))
	}

	total := int(final.Height-fromHeight) + 1
	finalRemoved := 0

	// Iterate from highest to lowest so that any interruption leaves a contiguous
	// range intact (no gaps between remaining heights).
	for height := final.Height; height >= fromHeight; height-- {
		head, err := protoState.AtHeight(height).Head()
		if err != nil {
			return nil, fmt.Errorf("could not get header at height %v: %w", height, err)
		}

		chunkIDs, err := RemoveExecutionResultsForBlock(
			protocolDBBatch, commits, transactionResults, results,
			chunkDataPacks, myReceipts, events, serviceEvents, head.ID())
		if err != nil {
			return nil, fmt.Errorf("could not remove result for finalized block at height %v: %w", height, err)
		}

		allChunkIDs = append(allChunkIDs, chunkIDs...)
		finalRemoved++
		log.Info().Msgf("removed result at height %v (%v/%v)", height, finalRemoved, total)
	}

	log.Info().Msgf("removed execution results from height %v: %v finalized, %v pending blocks",
		fromHeight, finalRemoved, len(pendings))

	return allChunkIDs, nil
}

// RemoveExecutionResultsForBlock removes all execution-related storage entries for a
// single block in a single batch write and returns the chunk IDs that should be
// removed from the chunk-data-pack database.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if the execution result for the block is absent (treated
//     as a no-op — the block was never executed)
func RemoveExecutionResultsForBlock(
	protocolDBBatch storage.Batch,
	commits storage.Commits,
	transactionResults storage.TransactionResults,
	results storage.ExecutionResults,
	chunks storage.ChunkDataPacks,
	myReceipts storage.MyExecutionReceipts,
	events storage.Events,
	serviceEvents storage.ServiceEvents,
	blockID flow.Identifier,
) ([]flow.Identifier, error) {
	result, err := results.ByBlockID(blockID)
	if errors.Is(err, storage.ErrNotFound) {
		log.Info().Msgf("no execution result for block %v — skipping", blockID)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get execution result for block %v: %w", blockID, err)
	}

	chunkIDs := make([]flow.Identifier, 0, len(result.Chunks))
	for _, chunk := range result.Chunks {
		chunkIDs = append(chunkIDs, chunk.ID())
	}

	if err = commits.BatchRemoveByBlockID(blockID, protocolDBBatch); err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("could not remove state commitment for block %v: %w", blockID, err)
		}
		log.Warn().Msgf("state commitment not found for block %v", blockID)
	}

	if err = transactionResults.BatchRemoveByBlockID(blockID, protocolDBBatch); err != nil {
		return nil, fmt.Errorf("could not remove transaction results for block %v: %w", blockID, err)
	}

	if err = myReceipts.BatchRemoveIndexByBlockID(blockID, protocolDBBatch); err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("could not remove own receipt index for block %v: %w", blockID, err)
		}
		log.Warn().Msgf("own receipt not found for block %v", blockID)
	}

	if err = events.BatchRemoveByBlockID(blockID, protocolDBBatch); err != nil {
		return nil, fmt.Errorf("could not remove events for block %v: %w", blockID, err)
	}

	if err = serviceEvents.BatchRemoveByBlockID(blockID, protocolDBBatch); err != nil {
		return nil, fmt.Errorf("could not remove service events for block %v: %w", blockID, err)
	}

	if err = results.BatchRemoveIndexByBlockID(blockID, protocolDBBatch); err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("could not remove execution result index for block %v: %w", blockID, err)
		}
		log.Warn().Msgf("execution result index not found for block %v", blockID)
	}

	return chunkIDs, nil
}

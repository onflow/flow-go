package pruner

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/block_iterator"
	"github.com/onflow/flow-go/module/block_iterator/executor"
	"github.com/onflow/flow-go/module/block_iterator/latest"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
)

const NextHeightForUnprunedExecutionDataPackKey = "NextHeightForUnprunedExecutionDataPackKey"

func LoopPruneExecutionDataFromRootToLatestSealed(
	ctx context.Context,
	log zerolog.Logger,
	metrics module.ExecutionMetrics,
	state protocol.State,
	protocolDB storage.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	chunkDataPacksDB *pebble.DB,
	config PruningConfig,
) error {

	chunksDB := pebbleimpl.ToDB(chunkDataPacksDB)
	// the creator can be reused to create new block iterator that can iterate from the last
	// checkpoint to the new latest (sealed) block.
	creator, getNextAndLatest, err := makeBlockIteratorCreator(state, protocolDB, headers, chunksDB, config)
	if err != nil {
		return err
	}

	pruner := NewChunkDataPackPruner(chunkDataPacks, results)

	// iterateAndPruneAll takes a block iterator and iterates through all the blocks
	// and decides how to prune the chunk data packs.
	iterateAndPruneAll := func(iter module.BlockIterator) error {
		err := executor.IterateExecuteAndCommitInBatch(
			ctx, log, metrics, iter, pruner, chunksDB, config.BatchSize, config.SleepAfterEachBatchCommit)
		if err != nil {
			return fmt.Errorf("failed to iterate, execute, and commit in batch: %w", err)
		}
		return nil
	}

	for {
		nextToPrune, latestToPrune, err := getNextAndLatest()
		if err != nil {
			return fmt.Errorf("failed to get next and latest to prune: %w", err)
		}

		commitDuration := 2 * time.Millisecond // with default batch size 1200, the avg commit duration is 2ms
		batchCount, totalDuration := EstimateBatchProcessing(
			nextToPrune, latestToPrune,
			config.BatchSize, config.SleepAfterEachBatchCommit, commitDuration)

		log.Info().
			Uint64("nextToPrune", nextToPrune).
			Uint64("latestToPrune", latestToPrune).
			Uint64("batchCount", batchCount).
			Str("totalDuration", totalDuration.String()).
			Msgf("execution data pruning will start in %s at %s, complete at %s",
				config.SleepAfterEachIteration,
				time.Now().Add(config.SleepAfterEachIteration).UTC(),
				time.Now().Add(config.SleepAfterEachIteration).Add(totalDuration).UTC(),
			)

		// report the target pruned height
		metrics.ExecutionTargetChunkDataPackPrunedHeight(latestToPrune)

		select {
		case <-ctx.Done():
			return nil
			// wait first so that we give the data pruning lower priority compare to other tasks.
			// also we can disable this feature by setting the sleep time to a very large value.
			// also allows the pruner to be more responsive to the context cancellation, meaning
			// while the pruner is sleeping, it can be cancelled immediately.
		case <-time.After(config.SleepAfterEachIteration):
		}

		iter, hasNext, err := creator.Create()
		if err != nil {
			return fmt.Errorf("failed to create block iterator: %w", err)
		}

		if !hasNext {
			// no more blocks to iterate, we are done.
			continue
		}

		err = iterateAndPruneAll(iter)
		if err != nil {
			return fmt.Errorf("failed to iterate, execute, and commit in batch: %w", err)
		}
	}
}

// makeBlockIteratorCreator create the block iterator creator
func makeBlockIteratorCreator(
	state protocol.State,
	protocolDB storage.DB,
	headers storage.Headers,
	chunkDataPacksDB storage.DB,
	config PruningConfig,
) (
	module.IteratorCreator,
	// this is for logging purpose, so that after each round of pruning,
	// we can log and report metrics about the next and latest to prune
	func() (nextToPrune uint64, latestToPrune uint64, err error),
	error, // any error are exception
) {
	root := state.Params().SealedRoot()
	sealedAndExecuted := latest.NewLatestSealedAndExecuted(
		root,
		state,
		protocolDB,
	)

	// retrieves the latest sealed and executed block height.
	// the threshold ensures that a certain number of blocks are retained for querying instead of being pruned.
	latest := &LatestPrunable{
		LatestSealedAndExecuted: sealedAndExecuted,
		threshold:               config.Threshold,
	}

	initializer := store.NewConsumerProgress(chunkDataPacksDB, NextHeightForUnprunedExecutionDataPackKey)

	creator, err := block_iterator.NewHeightBasedCreator(
		headers.BlockIDByHeight,
		initializer,
		root,
		latest.Latest,
	)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create height based block iterator creator: %w", err)
	}

	stateReader := creator.IteratorState()

	return creator, func() (nextToPrune uint64, latestToPrune uint64, err error) {
		next, err := stateReader.LoadState()
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get next height to prune: %w", err)
		}

		header, err := latest.Latest()
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get latest prunable block: %w", err)
		}

		return next, header.Height, nil
	}, nil
}

// estimateBatchProcessing estimates the number of batches and the total duration
// start, end are both inclusive
func EstimateBatchProcessing(
	start, end uint64,
	batchSize uint,
	sleepAfterEachBatchCommit time.Duration,
	commitDuration time.Duration,
) (
	batchCount uint64, totalDuration time.Duration) {
	if batchSize == 0 || start > end {
		return 0, 0
	}
	count := end - start + 1
	batchCount = (count + uint64(batchSize) - 1) / uint64(batchSize)

	totalDuration = time.Duration(batchCount-1)*sleepAfterEachBatchCommit + time.Duration(batchCount)*commitDuration

	return batchCount, totalDuration
}

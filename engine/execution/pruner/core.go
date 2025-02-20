package pruner

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
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
	badgerDB *badger.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	chunkDataPacksDB *pebble.DB,
	config PruningConfig,
) error {

	chunksDB := pebbleimpl.ToDB(chunkDataPacksDB)
	// the creator can be reused to create new block iterator that can iterate from the last
	// checkpoint to the new latest (sealed) block.
	creator, getNextAndLatest, err := makeBlockIteratorCreator(state, badgerDB, headers, chunksDB, config)
	if err != nil {
		return err
	}

	// the returned iterateAndPruneAll takes a block iterator and iterates through all the blocks
	// and decides how to prune the chunk data packs.
	iterateAndPruneAll := makeIterateAndPruneAll(
		log,
		ctx, // for cancelling the iteration when the context is done
		config,
		chunksDB,
		NewChunkDataPackPruner(chunkDataPacks, results),
	)

	for {
		nextToPrune, latestToPrune, err := getNextAndLatest()
		if err != nil {
			return fmt.Errorf("failed to get next and latest to prune: %w", err)
		}

		log.Info().
			Uint64("nextToPrune", nextToPrune).
			Uint64("latestToPrune", latestToPrune).
			Msgf("execution data pruning will start in %s at %s",
				config.SleepAfterEachIteration, time.Now().Add(config.SleepAfterEachIteration).UTC())

			// last pruned is nextToPrune - 1.
			// it won't underflow, because nextToPrune starts from root + 1
		metrics.ExecutionLastChunkDataPackPrunedHeight(nextToPrune - 1)

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
	badgerDB *badger.DB,
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
		badgerDB,
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

// makeIterateAndPruneAll takes config and chunk data packs db and pruner and returns a function that
// takes a block iterator and iterates through all the blocks and decides how to prune the chunk data packs.
func makeIterateAndPruneAll(
	log zerolog.Logger, ctx context.Context, config PruningConfig, chunkDataPacksDB storage.DB, prune *ChunkDataPackPruner,
) func(iter module.BlockIterator) error {
	isBatchFull := func(counter int) bool {
		return uint(counter) >= config.BatchSize
	}

	sleeper := func() {
		// if the context is done, return immediately
		// otherwise sleep for the configured time
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(config.SleepAfterEachBatchCommit):
			}
		}
	}

	return func(iter module.BlockIterator) error {
		err := executor.IterateExecuteAndCommitInBatch(log, ctx, iter, prune, chunkDataPacksDB, isBatchFull, sleeper)
		if err != nil {
			return fmt.Errorf("failed to iterate, execute, and commit in batch: %w", err)
		}
		return nil
	}
}

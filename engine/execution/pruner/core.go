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
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

const NextHeightForUnprunedExecutionDataPackKey = "NextHeightForUnprunedExecutionDataPackKey"

func LoopPruneExecutionDataFromRootToLatestSealed(
	log zerolog.Logger,
	ctx context.Context,
	state protocol.State,
	badgerDB *badger.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	chunkDataPacksDB *pebble.DB,
	config PruningConfig,
) error {
	// the creator can be reused to create new block iterator that can iterate from the last
	// checkpoint to the new latest (sealed) block.
	creator, getLatest, err := makeBlockIteratorCreator(state, badgerDB, headers, chunkDataPacksDB, config)
	if err != nil {
		return err
	}

	// the returned iterateAndPruneAll takes a block iterator and iterates through all the blocks
	// and decides how to prune the chunk data packs.
	iterateAndPruneAll := makeIterateAndPruneAll(
		log,
		ctx, // for cancelling the iteration when the context is done
		config,
		chunkDataPacksDB,
		NewChunKDataPackPruner(chunkDataPacks, results),
	)

	for {
		latest, err := getLatest.Latest()
		if err != nil {
			return fmt.Errorf("failed to get latest sealed and executed block: %w", err)
		}

		log.Info().
			Uint64("latest_height", latest.Height).
			Msgf("execution data pruning will start in %s at %s",
				config.SleepAfterEachIteration, time.Now().Add(config.SleepAfterEachIteration).UTC())

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
	chunkDataPacksDB *pebble.DB,
	config PruningConfig,
) (module.IteratorCreator, *LatestPrunable, error) {
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

	progress := pebblestorage.NewConsumerProgress(chunkDataPacksDB, NextHeightForUnprunedExecutionDataPackKey)

	creator, err := block_iterator.NewHeightBasedCreator(
		headers.BlockIDByHeight,
		progress,
		root,
		latest.Latest,
	)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create height based block iterator creator: %w", err)
	}

	return creator, latest, nil
}

// makeIterateAndPruneAll takes config and chunk data packs db and pruner and returns a function that
// takes a block iterator and iterates through all the blocks and decides how to prune the chunk data packs.
func makeIterateAndPruneAll(
	log zerolog.Logger, ctx context.Context, config PruningConfig, chunkDataPacksDB *pebble.DB, prune *ChunkDataPackPruner,
) func(iter module.BlockIterator) error {
	isBatchFull := func(counter int) bool {
		return uint(counter) >= config.BatchSize
	}

	sleeper := func() {
		time.Sleep(config.SleepAfterEachBatchCommit)
	}

	db := pebbleimpl.ToDB(chunkDataPacksDB)

	return func(iter module.BlockIterator) error {
		err := executor.IterateExecuteAndCommitInBatch(log, ctx, iter, prune, db, isBatchFull, sleeper)
		if err != nil {
			return fmt.Errorf("failed to iterate, execute, and commit in batch: %w", err)
		}
		return nil
	}
}

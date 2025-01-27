package pruner

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"

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
	ctx context.Context,
	state protocol.State,
	badgerDB *badger.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	chunkDataPacksDB *pebble.DB,
	config PruningConfig,
	callbackWhenOneIterationFinished func(),
) error {
	if config.BatchSize <= 0 {
		return fmt.Errorf("batch size must be greater than 0, %v", config.BatchSize)
	}

	isBatchFull := func(counter int) bool {
		return counter >= config.BatchSize
	}

	sleeper := func() {
		time.Sleep(config.SleepAfterEachBatchCommit)
	}

	root := state.Params().SealedRoot()
	sealed := latest.NewLatestSealedAndExecuted(
		root,
		state,
		badgerDB,
	)

	latest := &LatestPrunable{
		LatestSealedAndExecuted: sealed,
		threshold:               config.Threshold,
	}

	progress := pebblestorage.NewConsumerProgress(chunkDataPacksDB, NextHeightForUnprunedExecutionDataPackKey)

	// TODO: add real pruner
	prune := &ChunkDataPackPruner{
		chunkDataPacks: chunkDataPacks,
		results:        results,
	}

	creator, err := block_iterator.NewHeightBasedCreator(
		headers.BlockIDByHeight,
		progress,
		root,
		latest.Latest,
	)

	if err != nil {
		return fmt.Errorf("failed to create height based block iterator creator: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(config.SleepAfterEachIteration):
		}

		iter, err := creator.Create()
		if err != nil {
			return fmt.Errorf("failed to create block iterator: %w", err)
		}

		err = executor.IterateExecuteAndCommitInBatch(iter, prune, pebbleimpl.ToDB(chunkDataPacksDB), isBatchFull, sleeper)
		if err != nil {
			return fmt.Errorf("failed to iterate, execute, and commit in batch: %w", err)
		}

		// call the callback to report a completion of a pruning iteration
		callbackWhenOneIterationFinished()
	}
}

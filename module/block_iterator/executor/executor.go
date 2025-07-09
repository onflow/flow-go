package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// IterationExecutor allows the caller to customize the task to be executed for each block.
// for instance, the executor can prune data indexed by the block, or build another index for
// each iterated block.
type IterationExecutor interface {
	// ExecuteByBlockID executes the task for the block indexed by the blockID
	ExecuteByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) (exception error)
}

// IterateExecuteAndCommitInBatch iterates over blocks and execute tasks with data that was indexed by the block.
// the update to the storage database is done in batch, and the batch is committed when it's full.
// the iteration progress is saved after batch is committed, so that the iteration progress
// can be resumed after restart.
// it sleeps after each batch is committed in order to minimizing the impact on the system.
func IterateExecuteAndCommitInBatch(
	// ctx is used for cancelling the iteration when the context is done
	ctx context.Context,
	log zerolog.Logger,
	metrics module.ExecutionMetrics,
	// iterator decides how to iterate over blocks
	iter module.BlockIterator,
	// executor decides what data in the storage will be updated for a certain block
	executor IterationExecutor,
	// db creates a new batch for each block, and passed to the executor for adding updates,
	// the batch is commited when it's full
	db storage.DB,
	// batchSize decides the batch size for each commit.
	batchSize uint,
	// sleepAfterEachBatchCommit allows the caller to slow down the iteration after each batch is committed
	// in order to minimize the impact on the system
	sleepAfterEachBatchCommit time.Duration,
) error {
	noMoreBlocks := false

	for !noMoreBlocks {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		start := time.Now()
		iteratedCountInCurrentBatch := uint(0)

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for {
				// if the batch is full, commit it and enter the outer loop to
				// start a new batch
				if iteratedCountInCurrentBatch >= batchSize {
					return nil
				}

				blockID, hasNext, err := iter.Next()
				if err != nil {
					return err
				}
				if !hasNext {
					// no more blocks to iterate, we are done.
					// update the flag and prepare to exit the loop after committing the last batch
					noMoreBlocks = true
					return nil
				}

				err = executor.ExecuteByBlockID(blockID, rw)
				if err != nil {
					return err
				}

				iteratedCountInCurrentBatch++
			}
		})
		if err != nil {
			return err
		}

		// save the progress of the iteration, so that it can be resumed later
		_, err = iter.Checkpoint()
		if err != nil {
			return fmt.Errorf("failed to checkpoint iterator: %w", err)
		}

		// report the progress of the iteration
		startIndex, endIndex, nextIndex := iter.Progress()
		progress := CalculateProgress(startIndex, endIndex, nextIndex)

		log.Info().
			Str("commit-dur", time.Since(start).String()).
			Uint64("start-index", startIndex).
			Uint64("end-index", endIndex).
			Uint64("next-index", nextIndex).
			Str("progress", fmt.Sprintf("%.2f%%", progress)).
			Msg("batch committed")

		metrics.ExecutionLastChunkDataPackPrunedHeight(nextIndex - 1)

		// sleep after each batch commit to minimize the impact on the system
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(sleepAfterEachBatchCommit):
			// continue to next iteration
		}
	}

	return nil
}

// CalculateProgress calculates the progress of the iteration, it returns a percentage
// of the progress. [0, 100]
// start, end are both inclusive
func CalculateProgress(start, end, current uint64) float64 {
	if end < start {
		return 100.0
	}
	// If start == end, there is one more to process

	if current < start {
		return 0.0 // If current is below start, assume 0%
	}
	if current > end {
		return 100.0 // If current is above end, assume 100%
	}
	progress := float64(current-start) / float64(end-start) * 100.0
	if progress > 100.0 {
		return 100.0 // Cap at 100%
	}
	return progress
}

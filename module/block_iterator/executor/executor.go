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
	batch := db.NewBatch()
	defer func() {
		// Close last batch.
		// NOTE: batch variable is reused, so it refers to the last batch when defer is executed.
		batch.Close()
	}()

	iteratedCountInCurrentBatch := uint(0)

	startTime := time.Now()
	total := 0
	defer func() {
		log.Info().Str("duration", time.Since(startTime).String()).
			Int("total_block_executed", total).
			Msg("block iteration and execution completed")
	}()

	for {
		select {
		// when the context is done, commit the last batch and return
		case <-ctx.Done():
			if iteratedCountInCurrentBatch > 0 {
				// commit the last batch
				err := commitAndCheckpoint(log, metrics, batch, iter)
				if err != nil {
					return err
				}
			}
			return nil
		default:
		}

		// iterate over each block until the end
		blockID, hasNext, err := iter.Next()
		if err != nil {
			return err
		}

		if !hasNext {
			if iteratedCountInCurrentBatch > 0 {
				// commit last batch
				err := commitAndCheckpoint(log, metrics, batch, iter)
				if err != nil {
					return err
				}
			}

			break
		}

		// execute all the data indexed by the block
		err = executor.ExecuteByBlockID(blockID, batch)
		if err != nil {
			return fmt.Errorf("failed to execute by block ID %v: %w", blockID, err)
		}
		iteratedCountInCurrentBatch++
		total++

		// if batch is full, commit and sleep
		if iteratedCountInCurrentBatch >= batchSize {
			// commit the batch and save the progress
			err := commitAndCheckpoint(log, metrics, batch, iter)
			if err != nil {
				return err
			}

			// wait a bit to minimize the impact on the system
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(sleepAfterEachBatchCommit):
			}

			// Close previous batch before creating a new batch.
			batch.Close()

			// create a new batch, and reset iteratedCountInCurrentBatch
			batch = db.NewBatch()
			iteratedCountInCurrentBatch = 0
		}
	}

	return nil
}

// commitAndCheckpoint commits the batch and checkpoints the iterator
// so that the iteration progress can be resumed after restart.
func commitAndCheckpoint(
	log zerolog.Logger, metrics module.ExecutionMetrics, batch storage.Batch, iter module.BlockIterator) error {
	start := time.Now()

	err := batch.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	_, err = iter.Checkpoint()
	if err != nil {
		return fmt.Errorf("failed to checkpoint iterator: %w", err)
	}

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

package executor

import (
	"fmt"

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

// Sleeper allows the caller to slow down the iteration after each batch is committed
type Sleeper func()

// IsBatchFull decides the batch size for each commit.
// it takes the number of blocks iterated in the current batch,
// and returns whether the batch is full.
type IsBatchFull func(iteratedCountInCurrentBatch int) bool

// IterateExecuteAndCommitInBatch iterates over blocks and execute tasks with data that was indexed by the block.
// the update to the storage database is done in batch, and the batch is committed when it's full.
// the iteration progress is saved after batch is committed, so that the iteration progress
// can be resumed after restart.
// it sleeps after each batch is committed in order to minimizing the impact on the system.
func IterateExecuteAndCommitInBatch(
	// iterator decides how to iterate over blocks
	iter module.BlockIterator,
	// executor decides what data in the storage will be updated for a certain block
	executor IterationExecutor,
	// db creates a new batch for each block, and passed to the executor for adding updates,
	// the batch is commited when it's full
	db storage.DB,
	// isBatchFull decides the batch size for each commit.
	isBatchFull IsBatchFull,
	// sleeper allows the caller to slow down the iteration after each batch is committed
	// in order to minimize the impact on the system
	sleeper Sleeper,
) error {
	batch := db.NewBatch()
	iteratedCountInCurrentBatch := 0

	for {
		// iterate over each block until the end
		blockID, hasNext, err := iter.Next()
		if err != nil {
			return err
		}

		if !hasNext {
			// commit last batch
			err := commitAndCheckpoint(batch, iter)
			if err != nil {
				return err
			}

			break
		}

		// prune all the data indexed by the block
		err = executor.ExecuteByBlockID(blockID, batch)
		if err != nil {
			return fmt.Errorf("failed to prune by block ID %v: %w", blockID, err)
		}
		iteratedCountInCurrentBatch++

		// if batch is full, commit and sleep
		if isBatchFull(iteratedCountInCurrentBatch) {
			// commit the batch and save the progress
			err := commitAndCheckpoint(batch, iter)
			if err != nil {
				return err
			}

			// wait a bit to minimize the impact on the system
			sleeper()

			// create a new batch, and reset iteratedCountInCurrentBatch
			batch = db.NewBatch()
			iteratedCountInCurrentBatch = 0
		}
	}

	return nil
}

// commitAndCheckpoint commits the batch and checkpoints the iterator
// so that the iteration progress can be resumed after restart.
func commitAndCheckpoint(batch storage.Batch, iter module.BlockIterator) error {
	err := batch.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	_, err = iter.Checkpoint()
	if err != nil {
		return fmt.Errorf("failed to checkpoint iterator: %w", err)
	}

	return nil
}

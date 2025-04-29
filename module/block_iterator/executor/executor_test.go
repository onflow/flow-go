package executor_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/block_iterator/executor"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// verify the executor is able to iterate through all blocks from the iterator.
func TestExecute(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		blockCount := 10

		// prepare data
		cdps := make([]*flow.ChunkDataPack, 0, blockCount)
		bs := make([]flow.Identifier, 0, blockCount)
		for i := 0; i < blockCount; i++ {
			cdp := unittest.ChunkDataPackFixture(unittest.IdentifierFixture())
			cdps = append(cdps, cdp)
			bs = append(bs, cdp.ChunkID)
		}

		pdb := pebbleimpl.ToDB(db)

		// store the chunk data packs to be pruned later
		for _, cdp := range cdps {
			sc := storage.ToStoredChunkDataPack(cdp)
			require.NoError(t, pdb.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertChunkDataPack(rw.Writer(), sc)
			}))
		}

		// it's ok the chunk ids is used as block ids, because the iterator
		// basically just iterate over identifiers
		iter := &iterator{blocks: bs}
		pr := &testExecutor{
			executeByBlockID: func(id flow.Identifier, batch storage.ReaderBatchWriter) error {
				return operation.RemoveChunkDataPack(batch.Writer(), id)
			},
		}

		// prune blocks
		batchSize := uint(3)
		nosleep := time.Duration(0)
		require.NoError(t, executor.IterateExecuteAndCommitInBatch(
			context.Background(), unittest.Logger(), metrics.NewNoopCollector(), iter, pr, pdb, batchSize, nosleep))

		// expect all blocks are pruned
		for _, b := range bs {
			reader, err := pdb.Reader()
			require.NoError(t, err)

			// verify they are pruned
			var c storage.StoredChunkDataPack
			err = operation.RetrieveChunkDataPack(reader, b, &c)
			require.True(t, errors.Is(err, storage.ErrNotFound), "expected ErrNotFound but got %v", err)
		}
	})
}

// verify the pruning can be interrupted and resumed
func TestExecuteCanBeResumed(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		blockCount := 10

		cdps := make([]*flow.ChunkDataPack, 0, blockCount)
		bs := make([]flow.Identifier, 0, blockCount)
		for i := 0; i < blockCount; i++ {
			cdp := unittest.ChunkDataPackFixture(unittest.IdentifierFixture())
			cdps = append(cdps, cdp)
			bs = append(bs, cdp.ChunkID)
		}

		pdb := pebbleimpl.ToDB(db)

		// store the chunk data packs to be pruned later
		for _, cdp := range cdps {
			sc := storage.ToStoredChunkDataPack(cdp)
			require.NoError(t, pdb.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertChunkDataPack(rw.Writer(), sc)
			}))
		}

		// it's ok the chunk ids is used as block ids, because the iterator
		// basically just iterate over identifiers
		iter := &iterator{blocks: bs}
		interrupted := fmt.Errorf("interrupted")
		pruneUntilInterrupted := &testExecutor{
			executeByBlockID: func(id flow.Identifier, batch storage.ReaderBatchWriter) error {
				// the 5th block will interrupt the pruning
				// since the 5th block belongs to the 2nd batch,
				// only the first batch is actually pruned,
				// which means blocks from 0 to 2 are pruned
				if id == bs[5] {
					return interrupted // return sentinel error to interrupt the pruning
				}
				return operation.RemoveChunkDataPack(batch.Writer(), id)
			},
		}

		// prune blocks until interrupted at block 5
		batchSize := uint(3)
		nosleep := time.Duration(0)
		err := executor.IterateExecuteAndCommitInBatch(
			context.Background(), unittest.Logger(), metrics.NewNoopCollector(), iter, pruneUntilInterrupted, pdb, batchSize, nosleep)
		require.True(t, errors.Is(err, interrupted), fmt.Errorf("expected %v but got %v", interrupted, err))

		// expect all blocks are pruned
		for i, b := range bs {

			// verify they are pruned
			var c storage.StoredChunkDataPack

			if i < 3 {
				reader, err := pdb.Reader()
				require.NoError(t, err)

				// the first 3 blocks in the first batch are pruned
				err = operation.RetrieveChunkDataPack(reader, b, &c)
				require.True(t, errors.Is(err, storage.ErrNotFound), "expected ErrNotFound for block %v but got %v", i, err)
				continue
			}

			// verify the remaining blocks are not pruned yet
			reader, err := pdb.Reader()
			require.NoError(t, err)

			require.NoError(t, operation.RetrieveChunkDataPack(reader, b, &c))
		}

		// now resume the pruning
		iterToAll := restoreBlockIterator(iter.blocks, iter.stored)

		pr := &testExecutor{
			executeByBlockID: func(id flow.Identifier, batch storage.ReaderBatchWriter) error {
				return operation.RemoveChunkDataPack(batch.Writer(), id)
			},
		}

		require.NoError(t, executor.IterateExecuteAndCommitInBatch(
			context.Background(), unittest.Logger(), metrics.NewNoopCollector(), iterToAll, pr, pdb, batchSize, nosleep))

		// verify all blocks are pruned
		for _, b := range bs {
			reader, err := pdb.Reader()
			require.NoError(t, err)

			var c storage.StoredChunkDataPack
			// the first 5 blocks are pruned
			err = operation.RetrieveChunkDataPack(reader, b, &c)
			require.True(t, errors.Is(err, storage.ErrNotFound), "expected ErrNotFound but got %v", err)
		}
	})
}

type iterator struct {
	blocks []flow.Identifier
	cur    int
	stored int
}

var _ module.BlockIterator = (*iterator)(nil)

func (b *iterator) Next() (flow.Identifier, bool, error) {
	if b.cur >= len(b.blocks) {
		return flow.Identifier{}, false, nil
	}

	id := b.blocks[b.cur]
	b.cur++
	return id, true, nil
}

func (b *iterator) Checkpoint() (uint64, error) {
	b.stored = b.cur
	return uint64(b.cur), nil
}

func (b *iterator) Progress() (uint64, uint64, uint64) {
	return 0, uint64(len(b.blocks) - 1), uint64(b.cur)
}

func restoreBlockIterator(blocks []flow.Identifier, stored int) *iterator {
	return &iterator{
		blocks: blocks,
		cur:    stored,
		stored: stored,
	}
}

type testExecutor struct {
	executeByBlockID func(id flow.Identifier, batchWriter storage.ReaderBatchWriter) error
}

var _ executor.IterationExecutor = (*testExecutor)(nil)

func (p *testExecutor) ExecuteByBlockID(id flow.Identifier, batchWriter storage.ReaderBatchWriter) error {
	return p.executeByBlockID(id, batchWriter)
}

func TestCalculateProgress(t *testing.T) {
	tests := []struct {
		start   uint64
		end     uint64
		current uint64
		want    float64
	}{
		{1, 100, 1, 0.0},     // Just started
		{1, 100, 50, 49.49},  // Midway
		{1, 100, 100, 100.0}, // Completed
		{1, 100, 150, 100.0}, // Exceeds end
		{1, 100, 0, 0.0},     // Below start
		{1, 1, 1, 0.0},       // Start = End
		{1, 1, 0, 0.0},       // Start = End, but below
		{1, 100, 10, 9.09},   // Early progress
		{1, 100, 99, 98.99},  // Near completion
	}

	for _, tt := range tests {
		got := executor.CalculateProgress(tt.start, tt.end, tt.current)
		if (got-tt.want) > 0.01 || (tt.want-got) > 0.01 { // Allow small floating-point errors
			t.Errorf("calculateProgress(%d, %d, %d) = %.2f; want %.2f", tt.start, tt.end, tt.current, got, tt.want)
		}
	}
}

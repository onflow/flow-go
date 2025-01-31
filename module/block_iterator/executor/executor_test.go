package executor_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/block_iterator/executor"
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

		sleeper := &sleeper{}

		// prune blocks
		batchSize := 3
		require.NoError(t, executor.IterateExecuteAndCommitInBatch(iter, pr, pdb, func(count int) bool { return count >= batchSize }, sleeper.Sleep))

		// expect all blocks are pruned
		for _, b := range bs {
			// verify they are pruned
			var c storage.StoredChunkDataPack
			err := operation.RetrieveChunkDataPack(pdb.Reader(), b, &c)
			require.True(t, errors.Is(err, storage.ErrNotFound), "expected ErrNotFound but got %v", err)
		}

		// the sleeper should be called 3 times, because blockCount blocks will be pruned in 3 batchs.
		require.Equal(t, blockCount/batchSize, sleeper.count)
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

		sleeper := &sleeper{}

		// prune blocks until interrupted at block 5
		batchSize := 3
		err := executor.IterateExecuteAndCommitInBatch(iter, pruneUntilInterrupted, pdb, func(count int) bool { return count >= batchSize }, sleeper.Sleep)
		require.True(t, errors.Is(err, interrupted), fmt.Errorf("expected %v but got %v", interrupted, err))

		// expect all blocks are pruned
		for i, b := range bs {
			// verify they are pruned
			var c storage.StoredChunkDataPack

			if i < 3 {
				// the first 3 blocks in the first batch are pruned
				err := operation.RetrieveChunkDataPack(pdb.Reader(), b, &c)
				require.True(t, errors.Is(err, storage.ErrNotFound), "expected ErrNotFound for block %v but got %v", i, err)
				continue
			}

			// verify the remaining blocks are not pruned yet
			require.NoError(t, operation.RetrieveChunkDataPack(pdb.Reader(), b, &c))
		}

		// the sleeper should be called once
		require.Equal(t, 5/batchSize, sleeper.count)

		// now resume the pruning
		iterToAll := restoreBlockIterator(iter.blocks, iter.stored)

		pr := &testExecutor{
			executeByBlockID: func(id flow.Identifier, batch storage.ReaderBatchWriter) error {
				return operation.RemoveChunkDataPack(batch.Writer(), id)
			},
		}

		require.NoError(t, executor.IterateExecuteAndCommitInBatch(iterToAll, pr, pdb, func(count int) bool { return count >= batchSize }, sleeper.Sleep))

		// verify all blocks are pruned
		for _, b := range bs {
			var c storage.StoredChunkDataPack
			// the first 5 blocks are pruned
			err := operation.RetrieveChunkDataPack(pdb.Reader(), b, &c)
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

func restoreBlockIterator(blocks []flow.Identifier, stored int) *iterator {
	return &iterator{
		blocks: blocks,
		cur:    stored,
		stored: stored,
	}
}

type sleeper struct {
	count int
}

func (s *sleeper) Sleep() {
	s.count++
}

type testExecutor struct {
	executeByBlockID func(id flow.Identifier, batchWriter storage.ReaderBatchWriter) error
}

var _ executor.IterationExecutor = (*testExecutor)(nil)

func (p *testExecutor) ExecuteByBlockID(id flow.Identifier, batchWriter storage.ReaderBatchWriter) error {
	return p.executeByBlockID(id, batchWriter)
}

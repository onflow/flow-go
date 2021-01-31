package match_test

import (
	"sync"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/verification/match"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// chunk can convert to job and converted back
func TestChunkToJob(t *testing.T) {
	block := unittest.BlockFixture()
	chunk := unittest.ChunkFixture(block.ID(), 0)
	actual := match.JobToChunk(match.ChunkToJob(chunk))
	require.Equal(t, chunk, actual)
}

// 1. if pushing 10 jobs to chunks queue, then engine will
// receive only 3 jobs
// 2. if pushing 10 jobs to chunks queue, and engine will
// call finish will all the jobs, then engine will process
// 10 jobs in total
// 3. pushing 100 jobs concurrently, could end up having 100
// jobs processed by the consumer
func TestProduceConsume(t *testing.T) {
	t.Parallel()

	t.Run("pushing 10 jobs to chunks queue, engine will only receive 3", func(t *testing.T) {
		called := make([]*flow.Chunk, 0)
		lock := &sync.Mutex{}
		neverFinish := func(finishProcessing match.FinishProcessing, chunk *flow.Chunk) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, chunk)
		}
		WithConsumer(t, neverFinish, func(consumer *match.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			block := unittest.BlockFixture()
			chunks := make([]*flow.Chunk, 0)
			for i := 0; i < 10; i++ {
				chunk := unittest.ChunkFixture(block.ID(), uint(i))
				chunksQueue.StoreChunk(chunk)
				chunks = append(chunks, chunk)
				consumer.Check()
			}

			// expect the mock engine receives 3 calls
			require.Equal(t, chunks[:3], called)
		})
	})
}

func WithConsumer(
	t *testing.T,
	process func(match.FinishProcessing, *flow.Chunk),
	withConsumer func(*match.ChunkConsumer, *storage.ChunksQueue),
) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(3)
		maxFinished := int64(8)

		processedIndex := storage.NewConsumeProgress(db, module.ConsumeProgressVerificationChunkIndex)
		chunksQueue := storage.NewChunksQueue(db, match.DefaultJobIndex)

		engine := &MockEngine{
			process: process,
		}

		consumer, _ := match.NewChunkConsumer(
			unittest.Logger(),
			processedIndex,
			chunksQueue,
			engine,
			maxProcessing,
			maxFinished,
		)

		withConsumer(consumer, chunksQueue)
	})
}

type MockEngine struct {
	finishProcessing match.FinishProcessing
	process          func(finishProcessing match.FinishProcessing, chunk *flow.Chunk)
}

func (e *MockEngine) ProcessMyChunk(chunk *flow.Chunk) {
	e.process(e.finishProcessing, chunk)
}

func (e *MockEngine) WithFinishProcessing(finishProcessing match.FinishProcessing) {
	e.finishProcessing = finishProcessing
}

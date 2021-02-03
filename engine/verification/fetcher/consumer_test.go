package fetcher_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// chunk can convert to job and converted back
func TestChunkToJob(t *testing.T) {
	block := unittest.BlockFixture()
	chunk := unittest.ChunkFixture(block.ID(), 0)
	actual := fetcher.JobToChunk(fetcher.ChunkToJob(chunk))
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

	t.Run("pushing 10 jobs receive 3", func(t *testing.T) {
		called := make([]*flow.Chunk, 0)
		lock := &sync.Mutex{}
		neverFinish := func(finishProcessing fetcher.FinishProcessing, chunk *flow.Chunk) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, chunk)
		}
		WithConsumer(t, neverFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			<-consumer.Ready()

			block := unittest.BlockFixture()
			chunks := make([]*flow.Chunk, 0)
			for i := 0; i < 10; i++ {
				chunk := unittest.ChunkFixture(block.ID(), uint(i))
				ok, err := chunksQueue.StoreChunk(chunk)
				require.NoError(t, err, fmt.Sprintf("chunk %v can't be stored", i))
				require.True(t, ok)
				chunks = append(chunks, chunk)
				consumer.Check() // notify the consumer
			}

			<-consumer.Done()

			// expect the mock engine receives 3 calls
			require.Equal(t, chunks[:3], called)
		})
	})

	t.Run("pushing 10 receive 10", func(t *testing.T) {
		called := make([]*flow.Chunk, 0)
		lock := &sync.Mutex{}
		alwaysFinish := func(finishProcessing fetcher.FinishProcessing, chunk *flow.Chunk) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, chunk)
			go finishProcessing.FinishProcessing(chunk.ID())
		}
		WithConsumer(t, alwaysFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			<-consumer.Ready()

			block := unittest.BlockFixture()
			chunks := make([]*flow.Chunk, 0)
			for i := 0; i < 10; i++ {
				chunk := unittest.ChunkFixture(block.ID(), uint(i))
				ok, err := chunksQueue.StoreChunk(chunk)
				require.NoError(t, err, fmt.Sprintf("chunk %v can't be stored", i))
				require.True(t, ok)
				chunks = append(chunks, chunk)
				consumer.Check() // notify the consumer
			}

			<-consumer.Done()
			// expect the mock engine receives all 10 calls
			require.Equal(t, chunks, called)
		})
	})

	t.Run("pushing 100 concurrently receive 100", func(t *testing.T) {
		called := make([]*flow.Chunk, 0)
		lock := &sync.Mutex{}
		alwaysFinish := func(finishProcessing fetcher.FinishProcessing, chunk *flow.Chunk) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, chunk)
			go finishProcessing.FinishProcessing(chunk.ID())
		}
		WithConsumer(t, alwaysFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			<-consumer.Ready()

			block := unittest.BlockFixture()

			total := atomic.NewUint32(0)
			blockID := block.ID()
			for i := 0; i < 100; i++ {
				go func(i int) {
					chunk := unittest.ChunkFixture(blockID, uint(i))
					ok, err := chunksQueue.StoreChunk(chunk)
					require.NoError(t, err, fmt.Sprintf("chunk %v can't be stored", i))
					require.True(t, ok)
					total.Inc()
					consumer.Check() // notify the consumer
				}(i)
			}

			time.Sleep(100 * time.Millisecond)

			<-consumer.Done()
			// expect the mock engine receives all 100 calls
			require.Equal(t, 100, int(total.Load()))
		})
	})
}

func WithConsumer(
	t *testing.T,
	process func(fetcher.FinishProcessing, *flow.Chunk),
	withConsumer func(*fetcher.ChunkConsumer, *storage.ChunksQueue),
) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(3)
		maxFinished := int64(8)

		processedIndex := storage.NewConsumeProgress(db, module.ConsumeProgressVerificationChunkIndex)
		chunksQueue := storage.NewChunksQueue(db)
		ok, err := chunksQueue.Init(fetcher.DefaultJobIndex)
		require.NoError(t, err)
		require.True(t, ok)

		engine := &MockEngine{
			process: process,
		}

		consumer := fetcher.NewChunkConsumer(
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
	finishProcessing fetcher.FinishProcessing
	process          func(finishProcessing fetcher.FinishProcessing, chunk *flow.Chunk)
}

func (e *MockEngine) ProcessMyChunk(chunk *flow.Chunk) {
	e.process(e.finishProcessing, chunk)
}

func (e *MockEngine) WithFinishProcessing(finishProcessing fetcher.FinishProcessing) {
	e.finishProcessing = finishProcessing
}

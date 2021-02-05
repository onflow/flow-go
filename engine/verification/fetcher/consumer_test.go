package fetcher_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/module"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// chunk can convert to job and converted back
func TestChunkToJob(t *testing.T) {
	locator := &chunks.ChunkLocator{
		ResultID: unittest.IdentifierFixture(),
		Index:    rand.Uint64(),
	}
	actual := fetcher.JobToChunk(fetcher.ChunkLocatorToJob(locator))
	require.Equal(t, locator, actual)
}

// TestProduceConsume evaluates that on a consumer with 3 workers:
// 1. if pushing 10 jobs to chunks queue, then engine will receive only 3 jobs.
// 2. if pushing 10 jobs to chunks queue, and engine will call finish will all the jobs, then engine will process 10 jobs in total.
// 3. pushing 100 jobs concurrently, could end up having 100 jobs processed by the consumer.
func TestProduceConsume(t *testing.T) {
	t.Parallel()

	// if pushing 10 jobs to chunks queue, then engine will receive only 3 jobs, since consumer only has 3 workers.
	t.Run("pushing 10 jobs receive 3", func(t *testing.T) {
		called := make([]*chunks.ChunkLocator, 0)
		lock := &sync.Mutex{}
		neverFinish := func(finishProcessing fetcher.FinishProcessing, locator *chunks.ChunkLocator) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, locator)
		}
		WithConsumer(t, neverFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunkLocatorQueue) {
			<-consumer.Ready()

			locators := make([]*chunks.ChunkLocator, 0)
			resultID := unittest.IdentifierFixture()

			for i := 0; i < 10; i++ {
				chunkID := unittest.IdentifierFixture()
				chunkLocator := &chunks.ChunkLocator{
					ResultID: resultID,
					Index:    uint64(i),
				}

				ok, err := chunksQueue.StoreChunkLocator(chunkID, chunkLocator)
				require.NoError(t, err, fmt.Sprintf("chunk locator %v can't be stored", i))
				require.True(t, ok)
				locators = append(locators, chunkLocator)
				consumer.Check() // notify the consumer
			}

			<-consumer.Done()

			// expect the mock engine receives 3 calls
			require.Equal(t, locators[:3], called)
		})
	})

	//t.Run("pushing 10 receive 10", func(t *testing.T) {
	//	called := make([]*flow.Chunk, 0)
	//	lock := &sync.Mutex{}
	//	alwaysFinish := func(finishProcessing fetcher.FinishProcessing, chunk *flow.Chunk) {
	//		lock.Lock()
	//		defer lock.Unlock()
	//		called = append(called, chunk)
	//		go finishProcessing.FinishProcessing(chunk.ID())
	//	}
	//	WithConsumer(t, alwaysFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunkLocatorQueue) {
	//		<-consumer.Ready()
	//
	//		block := unittest.BlockFixture()
	//		chunks := make([]*flow.Chunk, 0)
	//		for i := 0; i < 10; i++ {
	//			chunk := unittest.ChunkFixture(block.ID(), uint(i))
	//			ok, err := chunksQueue.StoreChunkLocator(chunk)
	//			require.NoError(t, err, fmt.Sprintf("chunk %v can't be stored", i))
	//			require.True(t, ok)
	//			chunks = append(chunks, chunk)
	//			consumer.Check() // notify the consumer
	//		}
	//
	//		<-consumer.Done()
	//		// expect the mock engine receives all 10 calls
	//		require.Equal(t, chunks, called)
	//	})
	//})

	//t.Run("pushing 100 concurrently receive 100", func(t *testing.T) {
	//	called := make([]*chunks.ChunkLocator, 0)
	//	lock := &sync.Mutex{}
	//	alwaysFinish := func(finishProcessing fetcher.FinishProcessing, locator *chunks.ChunkLocator) {
	//		lock.Lock()
	//		defer lock.Unlock()
	//		called = append(called, locator)
	//		go finishProcessing.FinishProcessing(locator.ID())
	//	}
	//	WithConsumer(t, alwaysFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunkLocatorQueue) {
	//		<-consumer.Ready()
	//
	//		block := unittest.BlockFixture()
	//
	//		total := atomic.NewUint32(0)
	//		blockID := block.ID()
	//		for i := 0; i < 100; i++ {
	//			go func(i int) {
	//				chunk := unittest.ChunkFixture(blockID, uint(i))
	//				ok, err := chunksQueue.StoreChunkLocator(chunk)
	//				require.NoError(t, err, fmt.Sprintf("chunk %v can't be stored", i))
	//				require.True(t, ok)
	//				total.Inc()
	//				consumer.Check() // notify the consumer
	//			}(i)
	//		}
	//
	//		// expect the mock engine receives all 100 calls
	//		require.Eventually(t, func() bool {
	//			return int(total.Load()) == 100
	//		}, time.Second*10, time.Millisecond*10)
	//
	//		<-consumer.Done()
	//	})
	//})
}

func WithConsumer(
	t *testing.T,
	process func(fetcher.FinishProcessing, *chunks.ChunkLocator),
	withConsumer func(*fetcher.ChunkConsumer, *storage.ChunkLocatorQueue),
) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(3)
		maxFinished := int64(8)

		processedIndex := storage.NewConsumeProgress(db, module.ConsumeProgressVerificationChunkIndex)
		chunksQueue := storage.NewChunkLocatorQueue(db)
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
	process          func(finishProcessing fetcher.FinishProcessing, locator *chunks.ChunkLocator)
}

func (e *MockEngine) ProcessMyChunk(locator *chunks.ChunkLocator) {
	e.process(e.finishProcessing, locator)
}

func (e *MockEngine) WithFinishProcessing(finishProcessing fetcher.FinishProcessing) {
	e.finishProcessing = finishProcessing
}

package fetcher_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/module"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestChunkLocatorToJob evaluates that a chunk locator can be converted to a job, and its corresponding job can be converted back to a locator.
func TestChunkLocatorToJob(t *testing.T) {
	locator := &chunks.Locator{
		ResultID: unittest.IdentifierFixture(),
		Index:    rand.Uint64(),
	}
	actual := fetcher.JobToChunk(fetcher.ChunkLocatorToJob(locator))
	require.Equal(t, locator, actual)
}

// TestProduceConsume evaluates different scenarios on passing jobs to chunk queue with 3 workers on the consumer side. It evaluates blocking and
// none-blocking engines attacked to the workers in sequential and concurrent scenarios.
func TestProduceConsume(t *testing.T) {
	t.Parallel()

	// pushing 10 jobs sequentially to chunk queue, with 3 workers on consumer and the engine blocking on the jobs,
	// results in engine only receiving 3 jobs.
	t.Run("pushing 10 jobs receive 3", func(t *testing.T) {
		called := make([]*chunks.Locator, 0)
		lock := &sync.Mutex{}
		neverFinish := func(finishProcessing fetcher.FinishProcessing, locator *chunks.Locator) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, locator)
		}
		WithConsumer(t, neverFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			<-consumer.Ready()

			locators := make([]*chunks.Locator, 0)
			resultID := unittest.IdentifierFixture()

			for i := 0; i < 10; i++ {
				chunkLocator := &chunks.Locator{
					ResultID: resultID,
					Index:    uint64(i),
				}

				ok, err := chunksQueue.StoreChunkLocator(chunkLocator)
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

	// pushing 10 jobs sequentially to chunk queue, with 3 workers on consumer and the engine immediately finishing the job,
	// results in engine eventually receiving all 10 jobs.
	t.Run("pushing 10 receive 10", func(t *testing.T) {
		called := make([]*chunks.Locator, 0)
		lock := &sync.Mutex{}
		alwaysFinish := func(finishProcessing fetcher.FinishProcessing, locator *chunks.Locator) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, locator)
			go finishProcessing.FinishProcessing(locator.ID())
		}
		WithConsumer(t, alwaysFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			<-consumer.Ready()

			locators := make([]*chunks.Locator, 0)
			resultID := unittest.IdentifierFixture()

			for i := 0; i < 10; i++ {
				chunkLocator := &chunks.Locator{
					ResultID: resultID,
					Index:    uint64(i),
				}
				ok, err := chunksQueue.StoreChunkLocator(chunkLocator)
				require.NoError(t, err, fmt.Sprintf("chunk locator %v can't be stored", i))
				require.True(t, ok)
				locators = append(locators, chunkLocator)
				consumer.Check() // notify the consumer
			}

			<-consumer.Done()
			// expect the mock engine receives all 10 calls
			require.Equal(t, locators, called)
		})
	})

	// pushing 100 jobs concurrently to chunk queue, with 3 workers on consumer and the engine immediately finishing the job,
	// results in engine eventually receiving all 100 jobs.
	t.Run("pushing 100 concurrently receive 100", func(t *testing.T) {
		called := make([]*chunks.Locator, 0)
		lock := &sync.Mutex{}
		alwaysFinish := func(finishProcessing fetcher.FinishProcessing, locator *chunks.Locator) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, locator)
			go finishProcessing.FinishProcessing(locator.ID())
		}
		WithConsumer(t, alwaysFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			<-consumer.Ready()
			total := atomic.NewUint32(0)
			resultID := unittest.IdentifierFixture()

			for i := 0; i < 100; i++ {
				go func(i int) {
					chunkLocator := &chunks.Locator{
						ResultID: resultID,
						Index:    uint64(i),
					}
					ok, err := chunksQueue.StoreChunkLocator(chunkLocator)
					require.NoError(t, err, fmt.Sprintf("chunk locator %v can't be stored", i))
					require.True(t, ok)
					total.Inc()
					consumer.Check() // notify the consumer
				}(i)
			}

			// expect the mock engine receives all 100 calls
			require.Eventually(t, func() bool {
				return int(total.Load()) == 100
			}, time.Second*10, time.Millisecond*10)

			<-consumer.Done()
		})
	})
}

func WithConsumer(
	t *testing.T,
	process func(fetcher.FinishProcessing, *chunks.Locator),
	withConsumer func(*fetcher.ChunkConsumer, *storage.ChunksQueue),
) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(3)
		maxFinished := int64(8)

		processedIndex := storage.NewConsumeProgress(db, module.ConsumeProgressVerificationChunkIndex)
		chunksQueue := storage.NewChunkQueue(db)
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
	process          func(finishProcessing fetcher.FinishProcessing, locator *chunks.Locator)
}

func (e *MockEngine) ProcessMyChunk(locator *chunks.Locator) {
	e.process(e.finishProcessing, locator)
}

func (e *MockEngine) WithFinishProcessing(finishProcessing fetcher.FinishProcessing) {
	e.finishProcessing = finishProcessing
}

package fetcher_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/module"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestChunkLocatorToJob evaluates that a chunk locator can be converted to a job,
// and its corresponding job can be converted back to the same locator.
func TestChunkLocatorToJob(t *testing.T) {
	locator := unittest.ChunkLocatorFixture(unittest.IdentifierFixture(), rand.Uint64())
	actual, err := fetcher.JobToChunkLocator(fetcher.ChunkLocatorToJob(locator))
	require.NoError(t, err)
	require.Equal(t, locator, actual)
}

// TestProduceConsume evaluates different scenarios on passing jobs to chunk queue with 3 workers on the consumer side. It evaluates blocking and
// none-blocking engines attached to the workers in sequential and concurrent scenarios.
func TestProduceConsume(t *testing.T) {
	// pushing 10 jobs sequentially to chunk queue, with 3 workers on consumer and the engine blocking on the jobs,
	// results in engine only receiving 3 jobs.
	t.Run("pushing 10 jobs receive 3", func(t *testing.T) {
		called := chunks.LocatorList{}
		lock := &sync.Mutex{}
		neverFinish := func(notifier module.ProcessingNotifier, locator *chunks.Locator) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, locator)
		}
		WithConsumer(t, neverFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			<-consumer.Ready()

			locators := unittest.ChunkLocatorListFixture(10)

			for i, locator := range locators {
				ok, err := chunksQueue.StoreChunkLocator(locator)
				require.NoError(t, err, fmt.Sprintf("chunk locator %v can't be stored", i))
				require.True(t, ok)
				consumer.Check() // notify the consumer
			}

			<-consumer.Done()

			// expect the mock engine receive only the first 3 calls (since it is blocked on those, hence no
			// new job is fetched to process).
			require.Equal(t, locators[:3], called)
		})
	})

	// pushing 10 jobs sequentially to chunk queue, with 3 workers on consumer and the engine immediately finishing the job,
	// results in engine eventually receiving all 10 jobs.
	t.Run("pushing 10 receive 10", func(t *testing.T) {
		called := chunks.LocatorList{}
		lock := &sync.Mutex{}
		var finishAll sync.WaitGroup
		alwaysFinish := func(notifier module.ProcessingNotifier, locator *chunks.Locator) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, locator)
			finishAll.Add(1)
			go func() {
				notifier.Notify(locator.ID())
				finishAll.Done()
			}()
		}
		WithConsumer(t, alwaysFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			<-consumer.Ready()

			locators := unittest.ChunkLocatorListFixture(10)

			for i, locator := range locators {
				ok, err := chunksQueue.StoreChunkLocator(locator)
				require.NoError(t, err, fmt.Sprintf("chunk locator %v can't be stored", i))
				require.True(t, ok)
				consumer.Check() // notify the consumer
			}

			<-consumer.Done()
			finishAll.Wait() // wait until all finished
			// expect the mock engine receives all 10 calls
			require.Equal(t, locators, called)
		})
	})

	// pushing 100 jobs concurrently to chunk queue, with 3 workers on consumer and the engine immediately finishing the job,
	// results in engine eventually receiving all 100 jobs.
	t.Run("pushing 100 concurrently receive 100", func(t *testing.T) {
		called := chunks.LocatorList{}
		lock := &sync.Mutex{}
		var finishAll sync.WaitGroup
		finishAll.Add(100)
		alwaysFinish := func(notifier module.ProcessingNotifier, locator *chunks.Locator) {
			lock.Lock()
			defer lock.Unlock()
			called = append(called, locator)
			go func() {
				notifier.Notify(locator.ID())
				finishAll.Done()
			}()
		}
		WithConsumer(t, alwaysFinish, func(consumer *fetcher.ChunkConsumer, chunksQueue *storage.ChunksQueue) {
			<-consumer.Ready()
			total := atomic.NewUint32(0)

			locators := unittest.ChunkLocatorListFixture(100)

			for i := 0; i < len(locators); i++ {
				go func(i int) {
					ok, err := chunksQueue.StoreChunkLocator(locators[i])
					require.NoError(t, err, fmt.Sprintf("chunk locator %v can't be stored", i))
					require.True(t, ok)
					total.Inc()
					consumer.Check() // notify the consumer
				}(i)
			}

			finishAll.Wait()
			<-consumer.Done()

			// expect the mock engine receives all 100 calls
			require.Equal(t, uint32(100), total.Load())
		})
	})
}

func WithConsumer(
	t *testing.T,
	process func(module.ProcessingNotifier, *chunks.Locator),
	withConsumer func(*fetcher.ChunkConsumer, *storage.ChunksQueue),
) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(3)

		processedIndex := storage.NewConsumerProgress(db, module.ConsumeProgressVerificationChunkIndex)
		chunksQueue := storage.NewChunkQueue(db)
		ok, err := chunksQueue.Init(fetcher.DefaultJobIndex)
		require.NoError(t, err)
		require.True(t, ok)

		engine := &MockChunkProcessor{
			process: process,
		}

		consumer := fetcher.NewChunkConsumer(
			unittest.Logger(),
			processedIndex,
			chunksQueue,
			engine,
			maxProcessing,
		)

		withConsumer(consumer, chunksQueue)
	})
}

type MockChunkProcessor struct {
	notifier module.ProcessingNotifier
	process  func(notifier module.ProcessingNotifier, locator *chunks.Locator)
}

func (e *MockChunkProcessor) ProcessAssignedChunk(locator *chunks.Locator) {
	e.process(e.notifier, locator)
}

func (e *MockChunkProcessor) WithChunkConsumerNotifier(notifier module.ProcessingNotifier) {
	e.notifier = notifier
}

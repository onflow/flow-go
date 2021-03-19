package processor

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestBlockToJob evaluates that a block can be converted to a job,
// and its corresponding job can be converted back to the same block.
func TestBlockToJob(t *testing.T) {
	block := unittest.BlockFixture()
	actual := jobToBlock(blockToJob(&block))
	require.Equal(t, &block, actual)
}

// 1. if pushing 10 jobs to chunks queue, then engine will
// receive only 3 jobs
// 2. if pushing 10 jobs to chunks queue, and engine will
// call finish will all the jobs, then engine will process
// 10 jobs in total
// 3. pushing 100 jobs concurrently, could end up having 100
// jobs processed by the consumer
func TestProduceConsume(t *testing.T) {
	// pushing 10 finalized blocks sequentially to block reader, with 3 workers on consumer and the assigner engine blocking on the blocks,
	// results in engine only receiving the first three finalized blocks (in any order).
	t.Run("pushing 10 blocks, blocking, receives 3", func(t *testing.T) {
		received := make([]*flow.Block, 0)
		lock := &sync.Mutex{}
		neverFinish := func(notifier module.ProcessingNotifier, block *flow.Block) {
			lock.Lock()
			defer lock.Unlock()
			received = append(received, block)
		}

		WithConsumer(t, 10, 3, neverFinish, func(consumer *BlockConsumer, blocks []*flow.Block) {
			<-consumer.Ready()

			for i := 0; i < len(blocks); i++ {
				// consumer is only required to be "notified" that a new finalized block available.
				// It keeps track of the last finalized block it has read, and read the next height upon
				// getting notified as follows:
				consumer.OnFinalizedBlock(&model.Block{})
			}

			<-consumer.Done()

			// expect the mock engine receive only the first 3 calls (since it is blocked on those, hence no
			// new block is fetched to process).
			requireBlockListsEqualIgnoreOrder(t, received, blocks[:3])
		})
	})

	// pushing 10 finalized blocks sequentially to block reader, with 3 workers on consumer and the assigner engine finishes processing
	// all blocks immediately, results in engine receiving all 10 blocks (in any order).
	t.Run("pushing 10 blocks, non-blocking, receives 10", func(t *testing.T) {
		received := make([]*flow.Block, 0)
		lock := &sync.Mutex{}
		var processAll sync.WaitGroup
		alwaysFinish := func(notifier module.ProcessingNotifier, block *flow.Block) {
			lock.Lock()
			defer lock.Unlock()

			received = append(received, block)

			go func() {
				notifier.Notify(block.ID())
				processAll.Done()
			}()
		}

		WithConsumer(t, 10, 3, alwaysFinish, func(consumer *BlockConsumer, blocks []*flow.Block) {
			<-consumer.Ready()
			processAll.Add(len(blocks))

			for i := 0; i < len(blocks); i++ {
				// consumer is only required to be "notified" that a new finalized block available.
				// It keeps track of the last finalized block it has read, and read the next height upon
				// getting notified as follows:
				consumer.OnFinalizedBlock(&model.Block{})
			}

			// waits until all finished
			unittest.RequireReturnsBefore(t, processAll.Wait, 1*time.Second, "could not process all blocks on time")
			<-consumer.Done()

			// expects the mock engine receive all 10 blocks.
			requireBlockListsEqualIgnoreOrder(t, received, blocks)
		})
	})

	// pushing 100 finalized blocks concurrently to block reader, with 3 workers on consumer and the assigner engine finishes processing
	// all blocks immediately, results in engine receiving all 100 blocks (in any order).
	t.Run("pushing 100 blocks concurrently, non-blocking, receives 100", func(t *testing.T) {
		received := make([]*flow.Block, 0)
		lock := &sync.Mutex{}
		var processAll sync.WaitGroup
		alwaysFinish := func(notifier module.ProcessingNotifier, block *flow.Block) {
			lock.Lock()
			defer lock.Unlock()

			received = append(received, block)

			go func() {
				notifier.Notify(block.ID())
				processAll.Done()
			}()
		}

		WithConsumer(t, 100, 3, alwaysFinish, func(consumer *BlockConsumer, blocks []*flow.Block) {
			<-consumer.Ready()
			processAll.Add(len(blocks))

			for i := 0; i < len(blocks); i++ {
				go func() {
					// consumer is only required to be "notified" that a new finalized block available.
					// It keeps track of the last finalized block it has read, and read the next height upon
					// getting notified as follows:
					consumer.OnFinalizedBlock(&model.Block{})
				}()
			}

			// waits until all finished
			unittest.RequireReturnsBefore(t, processAll.Wait, 1*time.Second, "could not process all blocks on time")
			<-consumer.Done()

			// expects the mock engine receive all 100 blocks.
			requireBlockListsEqualIgnoreOrder(t, received, blocks)
		})
	})

}

func WithConsumer(
	t *testing.T,
	blockCount int,
	workerCount int,
	process func(notifier module.ProcessingNotifier, block *flow.Block),
	withConsumer func(*BlockConsumer, []*flow.Block),
) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(workerCount)

		processedHeight := bstorage.NewConsumerProgress(db, module.ConsumeProgressVerificationBlockHeight)
		collector := &metrics.NoopCollector{}
		tracer := &trace.NoopTracer{}
		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		s := testutil.CompleteStateFixture(t, collector, tracer, participants)

		engine := &MockAssignerEngine{
			process: process,
		}

		consumer, _, err := NewBlockConsumer(unittest.Logger(),
			processedHeight,
			s.Storage.Blocks,
			s.State,
			engine,
			maxProcessing)
		require.NoError(t, err)

		root, err := s.State.Params().Root()
		require.NoError(t, err)

		// generates 2 * blockCount chain of blocks in the form of R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
		// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
		// Container blocks only contain receipts of their preceding reference blocks. But they do not
		// hold any guarantees.
		results := utils.CompleteExecutionResultChainFixture(t, root, blockCount/2)
		blocks := extendStateWithFinalizedBlocks(t, results, s.State)
		// makes sure that we generated a block chain of requested length.
		require.Len(t, blocks, blockCount)

		withConsumer(consumer, blocks)
	})
}

func requireContainBlock(t *testing.T, block *flow.Block, blocks []*flow.Block) {
	blockID := block.ID()
	for _, b := range blocks {
		if b.ID() == blockID {
			return
		}
	}

	require.Fail(t, fmt.Sprintf("block %x is not in the list %v", blockID, blocks))
}

func requireBlockListsEqualIgnoreOrder(t *testing.T, src []*flow.Block, dst []*flow.Block) {
	require.Equal(t, len(src), len(dst), fmt.Sprintf("block lists are not of same length src: %d, dst: %d", len(src), len(dst)))
	for _, e := range src {
		requireContainBlock(t, e, dst)
	}
}

type MockAssignerEngine struct {
	notifier module.ProcessingNotifier
	process  func(module.ProcessingNotifier, *flow.Block)
}

func (e *MockAssignerEngine) ProcessFinalizedBlock(block *flow.Block) {
	e.process(e.notifier, block)
}

func (e *MockAssignerEngine) WithBlockConsumerNotifier(notifier module.ProcessingNotifier) {
	e.notifier = notifier
}

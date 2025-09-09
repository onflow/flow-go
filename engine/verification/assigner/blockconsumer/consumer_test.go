package blockconsumer_test

import (
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	vertestutils "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestBlockToJob evaluates that a block can be converted to a job,
// and its corresponding job can be converted back to the same block.
func TestBlockToJob(t *testing.T) {
	block := unittest.BlockFixture()
	actual, err := jobqueue.JobToBlock(jobqueue.BlockToJob(block))
	require.NoError(t, err)
	require.Equal(t, block, actual)
}

func TestProduceConsume(t *testing.T) {
	// pushing 10 finalized blocks sequentially to block reader, with 3 workers on consumer and the block processor
	// blocking on the blocks, results in processor only receiving the first three finalized blocks:
	// 10 blocks sequentially --> block reader --> consumer can read and push 3 blocks at a time to processor --> blocking processor.
	t.Run("pushing 10 blocks, blocking, receives 3", func(t *testing.T) {
		received := make([]*flow.Block, 0)
		lock := &sync.Mutex{}
		neverFinish := func(notifier module.ProcessingNotifier, block *flow.Block) {
			lock.Lock()
			defer lock.Unlock()
			received = append(received, block)

			// this processor never notifies consumer that it is done with the block.
			// hence from consumer perspective, it is blocking on each received block.
		}

		withConsumer(t, 10, 3, neverFinish, func(consumer *blockconsumer.BlockConsumer, blocks []*flow.Block) {
			unittest.RequireCloseBefore(t, consumer.Ready(), time.Second, "could not start consumer")

			for i := 0; i < len(blocks); i++ {
				// consumer is only required to be "notified" that a new finalized block available.
				// It keeps track of the last finalized block it has read, and read the next height upon
				// getting notified as follows:
				consumer.OnFinalizedBlock(&model.Block{})
			}

			unittest.RequireCloseBefore(t, consumer.Done(), time.Second, "could not terminate consumer")

			// expects the processor receive only the first 3 blocks (since it is blocked on those, hence no
			// new block is fetched to process).
			require.ElementsMatch(t, flow.GetIDs(blocks[:3]), flow.GetIDs(received))
		})
	})

	// pushing 10 finalized blocks sequentially to block reader, with 3 workers on consumer and the processor finishes processing
	// each received block immediately, results in processor receiving all 10 blocks:
	// 10 blocks sequentially --> block reader --> consumer can read and push 3 blocks at a time to processor --> processor finishes blocks
	// immediately.
	t.Run("pushing 100 blocks, non-blocking, receives 100", func(t *testing.T) {
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

		withConsumer(t, 100, 3, alwaysFinish, func(consumer *blockconsumer.BlockConsumer, blocks []*flow.Block) {
			unittest.RequireCloseBefore(t, consumer.Ready(), time.Second, "could not start consumer")
			processAll.Add(len(blocks))

			for i := 0; i < len(blocks); i++ {
				// consumer is only required to be "notified" that a new finalized block available.
				// It keeps track of the last finalized block it has read, and read the next height upon
				// getting notified as follows:
				consumer.OnFinalizedBlock(&model.Block{})
			}

			// waits until all blocks finish processing
			unittest.RequireReturnsBefore(t, processAll.Wait, time.Second, "could not process all blocks on time")
			unittest.RequireCloseBefore(t, consumer.Done(), time.Second, "could not terminate consumer")

			// expects the mock engine receive all 100 blocks.
			require.ElementsMatch(t, flow.GetIDs(blocks), flow.GetIDs(received))
		})
	})

}

// withConsumer is a test helper that sets up a block consumer with specified number of workers.
// The block consumer operates on a block reader with a chain of specified number of finalized blocks
// ready to read and consumer.
func withConsumer(
	t *testing.T,
	blockCount int,
	workerCount int,
	process func(notifier module.ProcessingNotifier, block *flow.Block),
	withBlockConsumer func(*blockconsumer.BlockConsumer, []*flow.Block),
) {

	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		maxProcessing := uint64(workerCount)

		processedHeight := store.NewConsumerProgress(pebbleimpl.ToDB(pdb), module.ConsumeProgressVerificationBlockHeight)
		collector := &metrics.NoopCollector{}
		tracer := trace.NewNoopTracer()
		log := unittest.Logger()
		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		s := testutil.CompleteStateFixture(t, log, collector, tracer, rootSnapshot)

		engine := &mockBlockProcessor{
			process: process,
		}

		consumer, _, err := blockconsumer.NewBlockConsumer(
			unittest.Logger(),
			collector,
			processedHeight,
			s.Storage.Blocks,
			s.State,
			engine,
			maxProcessing)
		require.NoError(t, err)

		// generates a chain of blocks in the form of root <- R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
		// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
		// Container blocks only contain receipts of their preceding reference blocks. But they do not
		// hold any guarantees.
		root, err := s.State.Final().Head()
		require.NoError(t, err)
		rootProtocolState, err := s.State.Final().ProtocolState()
		require.NoError(t, err)
		rootProtocolStateID := rootProtocolState.ID()
		clusterCommittee := participants.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
		sources := unittest.RandomSourcesFixture(110)
		results := vertestutils.CompleteExecutionReceiptChainFixture(
			t,
			root,
			rootProtocolStateID,
			blockCount/2,
			sources,
			vertestutils.WithClusterCommittee(clusterCommittee),
		)
		blocks := vertestutils.ExtendStateWithFinalizedBlocks(t, results, s.State)
		// makes sure that we generated a block chain of requested length.
		require.Len(t, blocks, blockCount)

		withBlockConsumer(consumer, blocks)
	})
}

// mockBlockProcessor provides a FinalizedBlockProcessor with a plug-and-play process method.
type mockBlockProcessor struct {
	notifier module.ProcessingNotifier
	process  func(module.ProcessingNotifier, *flow.Block)
}

func (e *mockBlockProcessor) ProcessFinalizedBlock(block *flow.Block) {
	e.process(e.notifier, block)
}

func (e *mockBlockProcessor) WithBlockConsumerNotifier(notifier module.ProcessingNotifier) {
	e.notifier = notifier
}

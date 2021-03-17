package blockconsumer

import (
	"sync"
	"testing"

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
	t.Run("pushing 10 blocks, receives 3", func(t *testing.T) {
		received := make([]*flow.Block, 0)
		lock := &sync.Mutex{}
		neverFinish := func(notifier module.ProcessingNotifier, block *flow.Block) {
			lock.Lock()
			defer lock.Unlock()
			received = append(received, block)
		}

		WithConsumer(t, 10, neverFinish, func(consumer *BlockConsumer, blocks []*flow.Block) {
			<-consumer.Ready()

			for i := 0; i < len(blocks); i++ {
				consumer.OnFinalizedBlock(&model.Block{})
			}

			<-consumer.Done()

			// expect the mock engine receive only the first 3 calls (since it is blocked on those, hence no
			// new block is fetched to process).
			require.Equal(t, blocks[:3], received)
		})

	})

}

func WithConsumer(
	t *testing.T,
	blockCount int,
	process func(notifier module.ProcessingNotifier, block *flow.Block),
	withConsumer func(*BlockConsumer, []*flow.Block),
) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(3)

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
		resultTestCases := utils.CompleteExecutionResultChainFixture(t, root, blockCount)
		blocks := make([]*flow.Block, 0)

		// extends protocol state with the chain of blocks.
		for _, result := range resultTestCases {
			err := s.State.Extend(result.ReferenceBlock)
			require.NoError(t, err)
			blocks = append(blocks, result.ReferenceBlock)

			err = s.State.Extend(result.ContainerBlock)
			require.NoError(t, err)
			blocks = append(blocks, result.ContainerBlock)
		}

		withConsumer(consumer, blocks)
	})
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

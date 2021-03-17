package blockconsumer

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/engine/verification/assigner"
	mockassigner "github.com/onflow/flow-go/engine/verification/assigner/mock"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
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
	withConsumer := func(*BlockConsumer, storage.Blocks, []*flow.Block) {}
	processor := &mockassigner.FinalizedBlockProcessor{}
	processor.On("WithBlockConsumerNotifier", mock.Anything).Return()
	WithConsumer(t, 1, processor, withConsumer)
}

func WithConsumer(
	t *testing.T,
	blockCount int,
	blockProcessor assigner.FinalizedBlockProcessor,
	withConsumer func(*BlockConsumer, storage.Blocks, []*flow.Block),
) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(3)

		processedHeight := bstorage.NewConsumerProgress(db, module.ConsumeProgressVerificationBlockHeight)
		collector := &metrics.NoopCollector{}
		tracer := &trace.NoopTracer{}
		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		s := testutil.CompleteStateFixture(t, collector, tracer, participants)

		consumer, _, err := NewBlockConsumer(unittest.Logger(),
			processedHeight,
			s.Storage.Blocks,
			s.State,
			blockProcessor,
			maxProcessing)
		require.NoError(t, err)

		root, err := s.State.Params().Root()
		require.NoError(t, err)

		resultTestCases := utils.CompleteExecutionResultChainFixture(t, root, blockCount)
		blocks := make([]*flow.Block, 0)

		for _, result := range resultTestCases {
			err := s.State.Extend(result.ReferenceBlock)
			require.NoError(t, err)
			blocks = append(blocks, result.ReferenceBlock)

			err = s.State.Extend(result.ContainerBlock)
			require.NoError(t, err)
			blocks = append(blocks, result.ContainerBlock)
		}

		withConsumer(consumer, s.Storage.Blocks, blocks)
	})
}

package blockconsumer

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockReader(t *testing.T) {
	WithTestSetup(t, 10, func(reader *FinalizedBlockReader, blocks []*flow.Block) {
		head, err := reader.Head()
		require.NoError(t, err)
		require.Equal(t, uint64(head), blocks[len(blocks)-1].Header.Height)
	})
}

func WithTestSetup(
	t *testing.T,
	blockCount int,
	withReader func(*FinalizedBlockReader, []*flow.Block),
) {
	require.Equal(t, blockCount%2, 0, "block count for this test should be even")
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collector := &metrics.NoopCollector{}
		tracer := &trace.NoopTracer{}
		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		s := testutil.CompleteStateFixture(t, collector, tracer, participants)

		reader := newFinalizedBlockReader(s.State, s.Storage.Blocks)

		root, err := s.State.Params().Root()
		require.NoError(t, err)

		// generates blockCount chain of blocks in the form of R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
		// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
		// Container blocks only contain receipts of their preceding reference blocks. But they do not
		// hold any guarantees.
		results := utils.CompleteExecutionResultChainFixture(t, root, blockCount/2)
		blocks := make([]*flow.Block, 0)

		// extends protocol state with the chain of blocks.
		for _, result := range results {
			err := s.State.Extend(result.ReferenceBlock)
			require.NoError(t, err)
			err = s.State.Finalize(result.ReferenceBlock.ID())
			require.NoError(t, err)

			blocks = append(blocks, result.ReferenceBlock)

			err = s.State.Extend(result.ContainerBlock)
			require.NoError(t, err)
			blocks = append(blocks, result.ContainerBlock)
			err = s.State.Finalize(result.ContainerBlock.ID())
			require.NoError(t, err)
		}

		withReader(reader, blocks)
	})
}

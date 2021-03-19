package processor

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestBlockReader evaluates that block reader correctly reads stored finalized blocks from the blocks storage and
// protocol state.
func TestBlockReader(t *testing.T) {
	WithTestSetup(t, 10, func(reader *FinalizedBlockReader, blocks []*flow.Block) {
		// head of block reader should be the same height as the last block on the chain.
		head, err := reader.Head()
		require.NoError(t, err)
		require.Equal(t, uint64(head), blocks[len(blocks)-1].Header.Height)

		// retrieved blocks from block reader should be the same as the original blocks stored in it.
		for _, actual := range blocks {
			index := int64(actual.Header.Height)
			job, err := reader.AtIndex(index)
			require.NoError(t, err)

			retrieved, err := toBlock(job)
			require.NoError(t, err)
			require.Equal(t, actual.ID(), retrieved.ID())
		}
	})
}

// WithTestSetup is a test helper that provides the implementation of its `withReader` with an instance of FinalizedBlockReader that is connected
// to blocks storage and protocol state. The protocol state has been extended with `blockCount` finalized blocks and those
// blocks are also provided for the `withReader` function body.
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
		blocks := extendStateWithFinalizedBlocks(t, results, s.State)

		withReader(reader, blocks)
	})
}

// extendStateWithFinalizedBlocks is a test helper to extend the execution state and return the list of blocks.
// It receives a list of complete execution result fixtures in the form of (R1 <- C1) <- (R2 <- C2) <- .....
// Where R and C are the reference and container blocks of a complete execution result fixture.
// Reference blocks contain guarantees, and container blocks contain execution receipt for their preceding reference block.
// Note: for sake of simplicity we do not include guarantees in the container blocks for now.
func extendStateWithFinalizedBlocks(t *testing.T, results []*utils.CompleteExecutionResult, state protocol.MutableState) []*flow.Block {
	blocks := make([]*flow.Block, 0)

	// extends protocol state with the chain of blocks.
	for _, result := range results {
		err := state.Extend(result.ReferenceBlock)
		require.NoError(t, err)
		err = state.Finalize(result.ReferenceBlock.ID())
		require.NoError(t, err)
		blocks = append(blocks, result.ReferenceBlock)

		err = state.Extend(result.ContainerBlock)
		require.NoError(t, err)
		err = state.Finalize(result.ContainerBlock.ID())
		require.NoError(t, err)
		blocks = append(blocks, result.ContainerBlock)
	}

	return blocks
}

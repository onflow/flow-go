package blockconsumer_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	vertestutils "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestBlockReader evaluates that block reader correctly reads stored finalized blocks from the blocks storage and
// protocol state.
func TestBlockReader(t *testing.T) {
	withReader(t, 10, func(reader *blockconsumer.FinalizedBlockReader, blocks []*flow.Block) {
		// head of block reader should be the same height as the last block on the chain.
		head, err := reader.Head()
		require.NoError(t, err)
		require.Equal(t, head, blocks[len(blocks)-1].Header.Height)

		// retrieved blocks from block reader should be the same as the original blocks stored in it.
		for _, actual := range blocks {
			index := actual.Header.Height
			job, err := reader.AtIndex(index)
			require.NoError(t, err)

			retrieved, err := blockconsumer.JobToBlock(job)
			require.NoError(t, err)
			require.Equal(t, actual.ID(), retrieved.ID())
		}
	})
}

// withReader is a test helper that sets up a block reader.
// It also provides a chain of specified number of finalized blocks ready to read by block reader, i.e., the protocol state is extended with the
// chain of blocks and the blocks are stored in blocks storage.
func withReader(
	t *testing.T,
	blockCount int,
	withBlockReader func(*blockconsumer.FinalizedBlockReader, []*flow.Block),
) {
	require.Equal(t, blockCount%2, 0, "block count for this test should be even")
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collector := &metrics.NoopCollector{}
		tracer := &trace.NoopTracer{}
		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		s := testutil.CompleteStateFixture(t, collector, tracer, rootSnapshot)

		reader := blockconsumer.NewFinalizedBlockReader(s.State, s.Storage.Blocks)

		// generates a chain of blocks in the form of root <- R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
		// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
		// Container blocks only contain receipts of their preceding reference blocks. But they do not
		// hold any guarantees.
		root, err := s.State.Params().Root()
		require.NoError(t, err)
		results := vertestutils.CompleteExecutionReceiptChainFixture(t, root, blockCount/2)
		blocks := vertestutils.ExtendStateWithFinalizedBlocks(t, results, s.State)

		withBlockReader(reader, blocks)
	})
}

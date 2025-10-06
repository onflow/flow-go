package jobqueue_test

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	vertestutils "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestBlockReader evaluates that block reader correctly reads stored finalized blocks from the blocks storage and
// protocol state.
func TestBlockReader(t *testing.T) {
	withReader(t, 10, func(reader *jobqueue.FinalizedBlockReader, blocks []*flow.Block) {
		// head of block reader should be the same height as the last block on the chain.
		head, err := reader.Head()
		require.NoError(t, err)
		require.Equal(t, head, blocks[len(blocks)-1].Height)

		// retrieved blocks from block reader should be the same as the original blocks stored in it.
		for _, actual := range blocks {
			index := actual.Height
			job, err := reader.AtIndex(index)
			require.NoError(t, err)

			retrieved, err := jobqueue.JobToBlock(job)
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
	withBlockReader func(*jobqueue.FinalizedBlockReader, []*flow.Block),
) {
	require.Equal(t, blockCount%2, 0, "block count for this test should be even")
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {

		collector := &metrics.NoopCollector{}
		tracer := trace.NewNoopTracer()
		log := unittest.Logger()
		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		s := testutil.CompleteStateFixture(t, log, collector, tracer, rootSnapshot)

		reader := jobqueue.NewFinalizedBlockReader(s.State, s.Storage.Blocks)

		// generates a chain of blocks in the form of root <- R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
		// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
		// Container blocks only contain receipts of their preceding reference blocks. But they do not
		// hold any guarantees.
		root, err := s.State.Final().Head()
		require.NoError(t, err)
		protocolState, err := s.State.Final().ProtocolState()
		require.NoError(t, err)
		protocolStateID := protocolState.ID()

		clusterCommittee := participants.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
		sources := unittest.RandomSourcesFixture(10)
		results := vertestutils.CompleteExecutionReceiptChainFixture(t, root, protocolStateID, blockCount/2, sources, vertestutils.WithClusterCommittee(clusterCommittee))
		blocks := vertestutils.ExtendStateWithFinalizedBlocks(t, results, s.State)

		withBlockReader(reader, blocks)
	})
}

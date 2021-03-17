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

func WithTestSetup(
	t *testing.T,
	blockCount int,
	withReader func(*FinalizedBlockReader, []*flow.Block),
) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collector := &metrics.NoopCollector{}
		tracer := &trace.NoopTracer{}
		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		s := testutil.CompleteStateFixture(t, collector, tracer, participants)

		reader := newFinalizedBlockReader(s.State, s.Storage.Blocks)

		root, err := s.State.Params().Root()
		require.NoError(t, err)

		// generates 2 * blockCount chain of blocks in the form of R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
		// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
		// Container blocks only contain receipts of their preceding reference blocks. But they do not
		// hold any guarantees.
		results := utils.CompleteExecutionResultChainFixture(t, root, blockCount)
		blocks := make([]*flow.Block, 0)

		// extends protocol state with the chain of blocks.
		for _, result := range results {
			err := s.State.Extend(result.ReferenceBlock)
			require.NoError(t, err)
			blocks = append(blocks, result.ReferenceBlock)

			err = s.State.Extend(result.ContainerBlock)
			require.NoError(t, err)
			blocks = append(blocks, result.ContainerBlock)
		}

		withReader(reader, blocks)
	})
}

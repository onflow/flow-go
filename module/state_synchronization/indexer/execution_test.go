package indexer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExecutionState_HeightByBlockID(t *testing.T) {
	blocks := buildBlocks(5)
	blocksByID := make(map[flow.Identifier]*flow.Block, 0)
	for _, b := range blocks {
		blocksByID[b.ID()] = b
	}

	indexer := ExecutionState{
		headers: synctest.MockBlockHeaderStorage(
			synctest.WithByID(blocksByID),
		),
	}

	for _, b := range blocks {
		ret, err := indexer.HeightByBlockID(b.ID())
		require.NoError(t, err)
		require.Equal(t, b.Header.Height, ret)
	}
}

func TestExecutionState_Commitment(t *testing.T) {
	const start, end = 10, 20

	t.Run("success", func(t *testing.T) {
		indexer := ExecutionState{
			commitments:       make(map[uint64]flow.StateCommitment),
			startIndexHeight:  start,
			lastIndexedHeight: counters.NewSequentialCounter(end),
		}

		commitments := indexCommitments(&indexer, start, end, t)
		for i := start; i <= end; i++ {
			ret, err := indexer.Commitment(uint64(i))
			require.NoError(t, err)
			assert.Equal(t, commitments[uint64(i)], ret)
		}
	})

	t.Run("invalid heights", func(t *testing.T) {
		indexer := ExecutionState{
			commitments:       make(map[uint64]flow.StateCommitment),
			startIndexHeight:  start,
			lastIndexedHeight: counters.NewSequentialCounter(end),
		}

		_ = indexCommitments(&indexer, start, end, t)
		tests := []struct {
			height uint64
			err    string
		}{
			{height: end + 1, err: "state commitment out of indexed height bounds, current height range: [10, 20], requested height: 21"},
			{height: start - 1, err: "state commitment out of indexed height bounds, current height range: [10, 20], requested height: 9"},
		}

		for i, test := range tests {
			c, err := indexer.Commitment(test.height)
			assert.Equal(t, flow.DummyStateCommitment, c)
			assert.EqualError(t, err, test.err, fmt.Sprintf("invalid height test number %d failed", i))
		}
	})
}

func indexCommitments(indexer *ExecutionState, start uint64, end uint64, t *testing.T) map[uint64]flow.StateCommitment {
	comms := make(map[uint64]flow.StateCommitment)
	for i := start; i <= end; i++ {
		commitment := flow.StateCommitment(unittest.IdentifierFixture())
		comms[i] = commitment
		err := indexer.indexCommitment(commitment, i)
		require.NoError(t, err)
	}
	return comms
}

func buildBlocks(n int) []*flow.Block {
	blocks := make([]*flow.Block, n)

	genesis := unittest.BlockFixture()
	blocks[0] = &genesis
	for i := 1; i < n; i++ {
		blocks[i] = unittest.BlockWithParentFixture(blocks[i-1].Header)
	}

	return blocks
}

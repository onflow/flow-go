package indexer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExecutionState_HeightByBlockID(t *testing.T) {
	blocks := generateBlocks(5)
	indexer := ExecutionState{headers: buildHeaders(blocks)}

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

type indexBlockDataTest struct {
	indexer         ExecutionState
	registers       *storagemock.Registers
	events          *storagemock.Events
	ctx             context.Context
	data            *execution_data.BlockExecutionDataEntity
	expectErr       error
	storeRegisters  func(t *testing.T, ID flow.Identifier, height uint64) error
	setLatestHeight func(t *testing.T, height uint64) error
	storeEvents     func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error
}

func (i *indexBlockDataTest) run(t *testing.T) {

	if i.storeRegisters != nil {
		i.registers.
			On("Store", mock.AnythingOfType("flow.RegisterEntries"), mock.AnythingOfType("uint64")).
			Return(func(ID flow.Identifier, height uint64) error {
				return i.storeRegisters(t, ID, height)
			})
	}

	if i.setLatestHeight != nil {
		i.registers.
			On("SetLatestHeight", mock.AnythingOfType("uint64")).
			Return(func(height uint64) error {
				return i.setLatestHeight(t, height)
			})
	}

	if i.storeEvents != nil {
		i.events.
			On("Store", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList")).
			Return(func(ID flow.Identifier, events []flow.EventsList) error {
				return i.storeEvents(t, ID, events)
			})
	}

	err := i.indexer.IndexBlockData(i.ctx, i.data)
	if i.expectErr != nil {
		assert.Equal(t, i.expectErr, err)
	} else {
		assert.NoError(t, err)
	}
}

func TestExecutionState_IndexBlockData(t *testing.T) {
	registers := storagemock.NewRegisters(t)
	events := storagemock.NewEvents(t)
	blocks := generateBlocks(1)
	headers := buildHeaders(blocks)
	block := blocks[0]
	start, end := block.Header.Height-5, block.Header.Height-1

	// test cases:
	// - no chunk data
	// - no registers data
	// - multiple inserts, same height
	// - smaller invalid height
	// - bigger invalid height
	// - error on register updates
	// - error on events
	// - full register data, events, collections...

	indexer := ExecutionState{
		registers:         registers,
		headers:           headers,
		events:            events,
		commitments:       make(map[uint64]flow.StateCommitment),
		startIndexHeight:  start,
		lastIndexedHeight: counters.NewSequentialCounter(end),
	}

	collection := unittest.CollectionFixture(5)
	ced := &execution_data.ChunkExecutionData{
		Collection: &collection,
		Events:     flow.EventsList{},
		TrieUpdate: testutils.TrieUpdateFixture(2, 1, 8),
	}

	bed := unittest.BlockExecutionDatEntityFixture(
		unittest.WithBlockExecutionDataBlockID(block.ID()),
		unittest.WithChunkExecutionDatas(ced),
	)

	test := indexBlockDataTest{
		indexer:   indexer,
		registers: registers,
		ctx:       context.Background(),
		data:      bed,
		setLatestHeight: func(t *testing.T, height uint64) error {
			assert.Equal(t, height, block.Header.Height)
			return nil
		},
		storeRegisters: func(t *testing.T, ID flow.Identifier, height uint64) error {
			assert.Equal(t, height, block.Header.Height)
			return nil
		},
		storeEvents: func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error {
			return nil
		},
	}

	test.run(t)

}

func indexCommitments(indexer *ExecutionState, start uint64, end uint64, t *testing.T) map[uint64]flow.StateCommitment {
	commits := generateCommitments(int(end - start))
	commitsHeight := make(map[uint64]flow.StateCommitment)

	for j, i := 0, start; i <= end; i++ {
		commitsHeight[i] = commits[j]
		err := indexer.indexCommitment(commits[j], i)
		require.NoError(t, err)
		j++
	}

	return commitsHeight
}

//func buildEvents() storage.Events {
//	events := storagemock.Events{}
//	events.
//		On("Store", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList")).
//		Return(func(id flow.Identifier, events []flow.EventsList) {
//
//	})
//}

func buildHeaders(blocks []*flow.Block) storage.Headers {
	blocksByID := make(map[flow.Identifier]*flow.Block, 0)
	for _, b := range blocks {
		blocksByID[b.ID()] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByID(blocksByID))
}

func generateCommitments(n int) []flow.StateCommitment {
	commits := make([]flow.StateCommitment, n)
	for i := 0; i < n; i++ {
		commits[i] = flow.StateCommitment(unittest.IdentifierFixture())
	}

	return commits
}

func generateBlocks(n int) []*flow.Block {
	blocks := make([]*flow.Block, n)

	genesis := unittest.BlockFixture()
	blocks[0] = &genesis
	for i := 1; i < n; i++ {
		blocks[i] = unittest.BlockWithParentFixture(blocks[i-1].Header)
	}

	return blocks
}

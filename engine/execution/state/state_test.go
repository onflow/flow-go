package state_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/storage/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func prepareTest(f func(t *testing.T, es state.ExecutionState)) func(*testing.T) {
	return func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(badgerDB *badger.DB) {
			unittest.RunWithTempDir(t, func(dbDir string) {
				ls, err := ledger.NewTrieStorage(dbDir)
				require.NoError(t, err)

				ctrl := gomock.NewController(t)

				stateCommitments := mocks.NewMockCommits(ctrl)

				stateCommitment := ls.EmptyStateCommitment()

				stateCommitments.EXPECT().ByBlockID(gomock.Any()).Return(stateCommitment, nil)

				chunkDataPacks := new(storage.ChunkDataPacks)

				executionResults := new(storage.ExecutionResults)

				es := state.NewExecutionState(ls, stateCommitments, chunkDataPacks, executionResults, badgerDB)

				f(t, es)
			})
		})
	}
}

func TestExecutionStateWithTrieStorage(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	registerID2 := make([]byte, 32)
	copy(registerID2, "vegetable")

	t.Run("commit write and read new state", prepareTest(func(t *testing.T, es state.ExecutionState) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(flow.Identifier{})
		assert.NoError(t, err)

		view1 := es.NewView(sc1)

		view1.Set(registerID1, flow.RegisterValue("apple"))
		view1.Set(registerID2, flow.RegisterValue("carrot"))

		sc2, err := es.CommitDelta(view1.Delta(), sc1)
		assert.NoError(t, err)

		view2 := es.NewView(sc2)

		b1, err := view2.Get(registerID1)
		assert.NoError(t, err)
		b2, err := view2.Get(registerID2)
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	}))

	t.Run("commit write and read previous state", prepareTest(func(t *testing.T, es state.ExecutionState) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(flow.Identifier{})
		assert.NoError(t, err)

		view1 := es.NewView(sc1)

		view1.Set(registerID1, flow.RegisterValue("apple"))

		sc2, err := es.CommitDelta(view1.Delta(), sc1)
		assert.NoError(t, err)

		// update value and get resulting state commitment
		view2 := es.NewView(sc2)
		view2.Set(registerID1, flow.RegisterValue("orange"))

		sc3, err := es.CommitDelta(view2.Delta(), sc2)
		assert.NoError(t, err)

		// create a view for previous state version
		view3 := es.NewView(sc2)

		// create a view for new state version
		view4 := es.NewView(sc3)

		// fetch the value at both versions
		b1, err := view3.Get(registerID1)
		assert.NoError(t, err)

		b2, err := view4.Get(registerID1)
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Equal(t, flow.RegisterValue("orange"), b2)
	}))

	t.Run("commit delete and read new state", prepareTest(func(t *testing.T, es state.ExecutionState) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(flow.Identifier{})
		assert.NoError(t, err)

		// set initial value
		view1 := es.NewView(sc1)
		view1.Set(registerID1, flow.RegisterValue("apple"))
		view1.Set(registerID2, flow.RegisterValue("apple"))

		sc2, err := es.CommitDelta(view1.Delta(), sc1)
		assert.NoError(t, err)

		// update value and get resulting state commitment
		view2 := es.NewView(sc2)
		view2.Delete(registerID1)

		sc3, err := es.CommitDelta(view2.Delta(), sc2)
		assert.NoError(t, err)

		// create a view for previous state version
		view3 := es.NewView(sc2)

		// create a view for new state version
		view4 := es.NewView(sc3)

		// fetch the value at both versions
		b1, err := view3.Get(registerID1)
		assert.NoError(t, err)

		b2, err := view4.Get(registerID1)
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Empty(t, b2)
	}))
}

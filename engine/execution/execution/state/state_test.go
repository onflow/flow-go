package state_test

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/storage/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func prepareTest(f func(t *testing.T, es state.ExecutionState)) func(*testing.T) {
	return func(t *testing.T) {
		unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {

			ls, err := ledger.NewTrieStorage(db)
			require.NoError(t, err)

			ctrl := gomock.NewController(t)

			stateCommitments := mocks.NewMockCommits(ctrl)

			stateCommitment := unittest.StateCommitmentFixture()

			stateCommitments.EXPECT().ByBlockID(gomock.Any()).Return(stateCommitment, nil)

			chunkHeaders := new(storage.ChunkHeaders)

			es := state.NewExecutionState(ls, stateCommitments, chunkHeaders)

			f(t, es)
		})
	}
}

func TestExecutionStateWithTrieStorage(t *testing.T) {
	t.Run("commit write and read new state", prepareTest(func(t *testing.T, es state.ExecutionState) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(flow.Identifier{})
		assert.NoError(t, err)

		view1 := es.NewView(sc1)

		view1.Set("fruit", []byte("apple"))
		view1.Set("vegetable", []byte("carrot"))

		sc2, err := es.CommitDelta(view1.Delta())
		assert.NoError(t, err)

		view2 := es.NewView(sc2)

		b1, err := view2.Get("fruit")
		assert.NoError(t, err)
		b2, err := view2.Get("vegetable")
		assert.NoError(t, err)

		assert.Equal(t, []byte("apple"), b1)
		assert.Equal(t, []byte("carrot"), b2)
	}))

	t.Run("commit write and read previous state", prepareTest(func(t *testing.T, es state.ExecutionState) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(flow.Identifier{})
		assert.NoError(t, err)

		view1 := es.NewView(sc1)

		view1.Set("fruit", []byte("apple"))

		sc2, err := es.CommitDelta(view1.Delta())
		assert.NoError(t, err)

		// update value and get resulting state commitment
		view2 := es.NewView(sc2)
		view2.Set("fruit", []byte("orange"))

		sc3, err := es.CommitDelta(view2.Delta())
		assert.NoError(t, err)

		// create a view for previous state version
		view3 := es.NewView(sc2)

		// create a view for new state version
		view4 := es.NewView(sc3)

		// fetch the value at both versions
		b1, err := view3.Get("fruit")
		assert.NoError(t, err)

		b2, err := view4.Get("fruit")
		assert.NoError(t, err)

		assert.Equal(t, []byte("apple"), b1)
		assert.Equal(t, []byte("orange"), b2)
	}))

	t.Run("commit delete and read new state", prepareTest(func(t *testing.T, es state.ExecutionState) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(flow.Identifier{})
		assert.NoError(t, err)

		// set initial value
		view1 := es.NewView(sc1)
		view1.Set("fruit", []byte("apple"))

		sc2, err := es.CommitDelta(view1.Delta())
		assert.NoError(t, err)

		// update value and get resulting state commitment
		view2 := es.NewView(sc2)
		view2.Delete("fruit")

		sc3, err := es.CommitDelta(view2.Delta())
		assert.NoError(t, err)

		// create a view for previous state version
		view3 := es.NewView(sc2)

		// create a view for new state version
		view4 := es.NewView(sc3)

		// fetch the value at both versions
		b1, err := view3.Get("fruit")
		assert.NoError(t, err)

		b2, err := view4.Get("fruit")
		assert.NoError(t, err)

		assert.Equal(t, []byte("apple"), b1)
		assert.Empty(t, b2)
	}))
}

func TestState_GetChunkRegisters(t *testing.T) {
	t.Run("non-existent chunk", func(t *testing.T) {
		ls := new(storage.Ledger)
		sc := new(storage.Commits)
		ch := new(storage.ChunkHeaders)

		chunkID := unittest.IdentifierFixture()

		ch.On("ByID", chunkID).Return(nil, fmt.Errorf("storage error"))

		es := state.NewExecutionState(ls, sc, ch)

		ledger, err := es.GetChunkRegisters(chunkID)
		assert.Nil(t, ledger)
		assert.Error(t, err)
	})

	t.Run("ledger storage error", func(t *testing.T) {
		ls := new(storage.Ledger)
		sc := new(storage.Commits)
		ch := new(storage.ChunkHeaders)

		chunkHeader := unittest.ChunkHeaderFixture()
		chunkID := chunkHeader.ChunkID

		registerIDs := chunkHeader.RegisterIDs

		ch.On("ByID", chunkID).Return(&chunkHeader, nil)
		ls.On("GetRegisters", registerIDs, chunkHeader.StartState).
			Return(nil, fmt.Errorf("storage error"))

		es := state.NewExecutionState(ls, sc, ch)

		registers, err := es.GetChunkRegisters(chunkID)
		assert.Error(t, err)
		assert.Nil(t, registers)

		ch.AssertExpectations(t)
		ls.AssertExpectations(t)
	})

	t.Run("existing chunk", func(t *testing.T) {
		ls := new(storage.Ledger)
		sc := new(storage.Commits)
		ch := new(storage.ChunkHeaders)

		chunkHeader := unittest.ChunkHeaderFixture()
		chunkID := chunkHeader.ChunkID

		registerIDs := chunkHeader.RegisterIDs
		registerValues := []flow.RegisterValue{{1}, {2}, {3}}

		ch.On("ByID", chunkID).Return(&chunkHeader, nil)
		ls.On("GetRegisters", registerIDs, chunkHeader.StartState).Return(registerValues, nil)

		es := state.NewExecutionState(ls, sc, ch)

		actualRegisters, err := es.GetChunkRegisters(chunkID)
		assert.NoError(t, err)

		expectedRegisters := flow.Ledger{
			string(registerIDs[0]): registerValues[0],
			string(registerIDs[1]): registerValues[1],
			string(registerIDs[2]): registerValues[2],
		}

		assert.Equal(t, expectedRegisters, actualRegisters)

		ch.AssertExpectations(t)
		ls.AssertExpectations(t)
	})
}

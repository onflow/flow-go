package state_test

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/storage/ledger"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/storage/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func prepareTest(f func(t *testing.T, es state.ExecutionState)) func(*testing.T) {
	return func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(badgerDB *badger.DB) {
			unittest.RunWithTempDBDir(t, func(dbDir string) {
				ls, err := ledger.NewTrieStorage(dbDir)
				require.NoError(t, err)

				ctrl := gomock.NewController(t)

				stateCommitments := mocks.NewMockCommits(ctrl)

				stateCommitment := ls.EmptyStateCommitment()

				stateCommitments.EXPECT().ByID(gomock.Any()).Return(stateCommitment, nil)

				chunkHeaders := new(storage.ChunkHeaders)

				chunkDataPacks := new(storage.ChunkDataPacks)

				executionResults := new(storage.ExecutionResults)

				es := state.NewExecutionState(ls, stateCommitments, chunkHeaders, chunkDataPacks, executionResults, badgerDB)

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

func TestState_GetChunkRegisters(t *testing.T) {
	t.Run("non-existent chunk", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			ls := new(storage.Ledger)
			sc := new(storage.Commits)
			ch := new(storage.ChunkHeaders)
			cdp := new(storage.ChunkDataPacks)

			er := new(storage.ExecutionResults)

			chunkID := unittest.IdentifierFixture()

			ch.On("ByID", chunkID).Return(nil, fmt.Errorf("storage error"))

			es := state.NewExecutionState(ls, sc, ch, cdp, er, db)

			ledger, err := es.GetChunkRegisters(chunkID)
			assert.Nil(t, ledger)
			assert.Error(t, err)

			ch.AssertExpectations(t)
			ls.AssertExpectations(t)
			sc.AssertExpectations(t)
			er.AssertExpectations(t)
		})
	})

	t.Run("ledger storage error", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			ls := new(storage.Ledger)
			sc := new(storage.Commits)
			ch := new(storage.ChunkHeaders)
			cdp := new(storage.ChunkDataPacks)

			er := new(storage.ExecutionResults)

			chunkHeader := unittest.ChunkHeaderFixture()
			chunkID := chunkHeader.ChunkID

			registerIDs := chunkHeader.RegisterIDs

			ch.On("ByID", chunkID).Return(&chunkHeader, nil)
			ls.On("GetRegisters", registerIDs, chunkHeader.StartState).
				Return(nil, fmt.Errorf("storage error"))

			es := state.NewExecutionState(ls, sc, ch, cdp, er, db)

			registers, err := es.GetChunkRegisters(chunkID)
			assert.Error(t, err)
			assert.Nil(t, registers)

			ch.AssertExpectations(t)
			ls.AssertExpectations(t)
			sc.AssertExpectations(t)
			er.AssertExpectations(t)
		})

	})

	t.Run("existing chunk", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			ls := new(storage.Ledger)
			sc := new(storage.Commits)
			ch := new(storage.ChunkHeaders)
			cdp := new(storage.ChunkDataPacks)

			er := new(storage.ExecutionResults)

			chunkHeader := unittest.ChunkHeaderFixture()
			chunkID := chunkHeader.ChunkID

			registerIDs := chunkHeader.RegisterIDs
			registerValues := []flow.RegisterValue{{1}, {2}, {3}}

			ch.On("ByID", chunkID).Return(&chunkHeader, nil)
			ls.On("GetRegisters", registerIDs, chunkHeader.StartState).Return(registerValues, nil)

			es := state.NewExecutionState(ls, sc, ch, cdp, er, db)

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
			sc.AssertExpectations(t)
			er.AssertExpectations(t)
		})
	})

	t.Run("FindLatestFinalizedAndExecutedBlock", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			ls := new(storage.Ledger)
			sc := new(storage.Commits)
			ch := new(storage.ChunkHeaders)
			cdp := new(storage.ChunkDataPacks)
			er := new(storage.ExecutionResults)
			es := state.NewExecutionState(ls, sc, ch, cdp, er, db)

			blockA := unittest.BlockFixture()
			blockB := unittest.BlockWithParentFixture(&blockA.Header)
			blockC := unittest.BlockWithParentFixture(&blockB.Header)
			blockD := unittest.BlockWithParentFixture(&blockC.Header)

			err := db.Update(func(txn *badger.Txn) error {
				err := procedure.InsertBlock(&blockA)(txn)
				require.NoError(t, err)

				err = procedure.InsertBlock(&blockB)(txn)
				require.NoError(t, err)

				err = procedure.InsertBlock(&blockC)(txn)
				require.NoError(t, err)

				err = procedure.InsertBlock(&blockD)(txn)
				require.NoError(t, err)

				err = operation.InsertNumber(blockA.View, blockA.ID())(txn)
				require.NoError(t, err)

				err = operation.InsertBoundary(blockA.View)(txn)
				require.NoError(t, err)

				err = procedure.FinalizeBlock(blockB.ID())(txn)
				require.NoError(t, err)

				err = procedure.FinalizeBlock(blockC.ID())(txn) //finalize up to C
				require.NoError(t, err)

				err = operation.IndexStateCommitment(blockA.ID(), unittest.StateCommitmentFixture())(txn)
				require.NoError(t, err)

				err = operation.IndexStateCommitment(blockB.ID(), unittest.StateCommitmentFixture())(txn) // state commitment up to B
				require.NoError(t, err)

				return nil
			})
			require.NoError(t, err)

			header, err := es.FindLatestFinalizedAndExecutedBlock()
			assert.NoError(t, err)

			assert.Equal(t, &blockB.Header, header)

			ch.AssertExpectations(t)
			ls.AssertExpectations(t)
			sc.AssertExpectations(t)
			er.AssertExpectations(t)
		})
	})
}

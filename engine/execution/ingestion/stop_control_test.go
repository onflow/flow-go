package ingestion

import (
	"context"
	"testing"

	"github.com/onflow/flow-go/storage"

	testifyMock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/mock"

	"github.com/onflow/flow-go/utils/unittest"
)

// If stopping mechanism has caused any changes to execution flow (skipping execution of blocks)
// we disallow setting new values
func TestCannotSetNewValuesAfterStoppingCommenced(t *testing.T) {

	t.Run("when processing block at stop height", func(t *testing.T) {
		sc := NewStopControl(unittest.Logger())

		require.Nil(t, sc.GetNextStop())

		// first update is always successful
		err := sc.SetStopHeight(21, false)
		require.NoError(t, err)

		// TODO: check value of next stop
		require.NotNil(t, sc.GetNextStop())

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sc.BlockProcessable(header)
		err = sc.SetStopHeight(37, false)
		require.NoError(t, err)

		// block at stop height, it should be skipped
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sc.BlockProcessable(header)

		// cannot set new stop height after stopping has started
		err = sc.SetStopHeight(2137, false)
		require.Error(t, err)

		// state did not change
		// TODO: check value of next stop
	})

	t.Run("when processing finalized blocks", func(t *testing.T) {

		execState := new(mock.ReadOnlyExecutionState)

		sc := NewStopControl(unittest.Logger())

		require.Nil(t, sc.GetNextStop())

		// first update is always successful
		err := sc.SetStopHeight(21, false)
		require.NoError(t, err)
		// TODO: check value of next stop
		require.NotNil(t, sc.GetNextStop())

		// make execution check pretends block has been executed
		execState.On("StateCommitmentByBlockID", testifyMock.Anything, testifyMock.Anything).Return(nil, nil)

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sc.BlockFinalized(context.TODO(), execState, header)

		err = sc.SetStopHeight(37, false)
		require.NoError(t, err)
		// TODO: check value of next stop
		require.NotNil(t, sc.GetNextStop())

		// block at stop height, it should be triggered stop
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sc.BlockFinalized(context.TODO(), execState, header)

		// since we set shouldCrash to false, execution should be stopped
		require.True(t, sc.IsExecutionStopped())

		err = sc.SetStopHeight(2137, false)
		require.Error(t, err)

		execState.AssertExpectations(t)
	})
}

// TestExecutionFallingBehind check if StopControl behaves properly even if EN runs behind
// and blocks are finalized before they are executed
func TestExecutionFallingBehind(t *testing.T) {

	execState := new(mock.ReadOnlyExecutionState)

	headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
	headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
	headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22
	headerD := unittest.BlockHeaderWithParentFixture(headerC) // 23

	sc := NewStopControl(unittest.Logger())

	// set stop at 22, so 21 is the last height which should be processed
	err := sc.SetStopHeight(22, false)
	require.NoError(t, err)
	// TODO: check value of next stop
	require.NotNil(t, sc.GetNextStop())

	execState.
		On("StateCommitmentByBlockID", testifyMock.Anything, headerC.ParentID).
		Return(nil, storage.ErrNotFound)

	// finalize blocks first
	sc.BlockFinalized(context.TODO(), execState, headerA)
	sc.BlockFinalized(context.TODO(), execState, headerB)
	sc.BlockFinalized(context.TODO(), execState, headerC)
	sc.BlockFinalized(context.TODO(), execState, headerD)

	// simulate execution
	sc.OnBlockExecuted(headerA)
	sc.OnBlockExecuted(headerB)
	require.True(t, sc.IsExecutionStopped())

	execState.AssertExpectations(t)
}

// StopControl started as paused will keep the state
func TestStartingPaused(t *testing.T) {

	sc := NewStopControl(unittest.Logger())
	sc.StopExecution()
	require.True(t, sc.IsExecutionStopped())
}

func TestPausedStateRejectsAllBlocksAndChanged(t *testing.T) {

	sc := NewStopControl(unittest.Logger())
	sc.StopExecution()
	require.True(t, sc.IsExecutionStopped())

	err := sc.SetStopHeight(2137, true)
	require.Error(t, err)

	// make sure we don't even query executed status if paused
	// mock should fail test on any method call
	execState := new(mock.ReadOnlyExecutionState)

	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))

	sc.BlockFinalized(context.TODO(), execState, header)
	require.True(t, sc.IsExecutionStopped())

	execState.AssertExpectations(t)
}

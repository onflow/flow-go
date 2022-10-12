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
		sc := NewStopControl(unittest.Logger(), false, 0)

		require.Equal(t, sc.GetState(), StopControlOff)

		// first update is always successful
		_, _, err := sc.SetStopHeight(21, false)
		require.NoError(t, err)

		require.Equal(t, sc.GetState(), StopControlSet)

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sc.BlockProcessable(header)

		require.Equal(t, sc.GetState(), StopControlSet)

		_, _, err = sc.SetStopHeight(37, false)
		require.NoError(t, err)

		// block at stop height, it should be skipped
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sc.BlockProcessable(header)

		require.Equal(t, sc.GetState(), StopControlCommenced)

		_, _, err = sc.SetStopHeight(2137, false)
		require.Error(t, err)

		// state did not change
		require.Equal(t, sc.GetState(), StopControlCommenced)
	})

	t.Run("when processing finalized blocks", func(t *testing.T) {

		execState := new(mock.ReadOnlyExecutionState)

		sc := NewStopControl(unittest.Logger(), false, 0)

		require.Equal(t, sc.GetState(), StopControlOff)

		// first update is always successful
		_, _, err := sc.SetStopHeight(21, false)
		require.NoError(t, err)
		require.Equal(t, sc.GetState(), StopControlSet)

		// make execution check pretends block has been executed
		execState.On("StateCommitmentByBlockID", testifyMock.Anything, testifyMock.Anything).Return(nil, nil)

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sc.BlockFinalized(context.TODO(), execState, header)

		_, _, err = sc.SetStopHeight(37, false)
		require.NoError(t, err)
		require.Equal(t, sc.GetState(), StopControlSet)

		// block at stop height, it should be trigger stop
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sc.BlockFinalized(context.TODO(), execState, header)

		// since we set crash to false, execution should be paused
		require.Equal(t, sc.GetState(), StopControlPaused)

		_, _, err = sc.SetStopHeight(2137, false)
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

	sc := NewStopControl(unittest.Logger(), false, 0)

	require.Equal(t, sc.GetState(), StopControlOff)

	// set stop at 22, so 21 is the last height which should be processed
	_, _, err := sc.SetStopHeight(22, false)
	require.NoError(t, err)
	require.Equal(t, sc.GetState(), StopControlSet)

	//execState.On("StateCommitmentByBlockID", mock2.Anything, headerA.ParentID).Return(nil, storage.ErrNotFound)
	//execState.On("StateCommitmentByBlockID", mock2.Anything, headerB.ParentID).Return(nil, storage.ErrNotFound)
	execState.On("StateCommitmentByBlockID", testifyMock.Anything, headerC.ParentID).Return(nil, storage.ErrNotFound)

	// finalize blocks first
	sc.BlockFinalized(context.TODO(), execState, headerA)
	require.Equal(t, StopControlSet, sc.GetState())

	sc.BlockFinalized(context.TODO(), execState, headerB)
	require.Equal(t, StopControlSet, sc.GetState())

	sc.BlockFinalized(context.TODO(), execState, headerC)
	require.Equal(t, StopControlSet, sc.GetState())

	sc.BlockFinalized(context.TODO(), execState, headerD)
	require.Equal(t, StopControlSet, sc.GetState())

	// simulate execution
	sc.BlockExecuted(headerA)
	require.Equal(t, StopControlSet, sc.GetState())

	sc.BlockExecuted(headerB)
	require.Equal(t, StopControlPaused, sc.GetState())

	execState.AssertExpectations(t)
}

// TestCannotSetHeightBelowLastExecuted check if StopControl
// tracks last executed height and prevents from setting stop height
// below or too close to it
func TestCannotSetHeightBelowLastExecuted(t *testing.T) {

	sc := NewStopControl(unittest.Logger(), false, 0)

	require.Equal(t, sc.GetState(), StopControlOff)

	headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
	sc.BlockExecuted(headerA)
	require.Equal(t, StopControlOff, sc.GetState())

	_, _, err := sc.SetStopHeight(20, false)
	require.Error(t, err)
	require.Equal(t, StopControlOff, sc.GetState())

	_, _, err = sc.SetStopHeight(25, false)
	require.NoError(t, err)
	require.Equal(t, StopControlSet, sc.GetState())
}

// StopControl started as paused will keep the state
func TestStartingPaused(t *testing.T) {

	sc := NewStopControl(unittest.Logger(), true, 0)
	require.Equal(t, StopControlPaused, sc.GetState())
}

func TestPausedStateRejectsAllBlocksAndChanged(t *testing.T) {

	sc := NewStopControl(unittest.Logger(), true, 0)
	require.Equal(t, StopControlPaused, sc.GetState())

	_, _, err := sc.SetStopHeight(2137, true)
	require.Error(t, err)

	// make sure we don't even query executed status if paused
	// mock should fail test on any method call
	execState := new(mock.ReadOnlyExecutionState)

	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))

	sc.BlockFinalized(context.TODO(), execState, header)
	require.Equal(t, StopControlPaused, sc.GetState())

	execState.AssertExpectations(t)
}

package ingestion

import (
	"context"
	"testing"

	"github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/rs/zerolog"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// If stopping mechanism has caused any changes to execution flow (skipping execution of blocks)
// we disallow setting new values
func TestCannotSetNewValuesAfterStoppingCommenced(t *testing.T) {

	t.Run("when processing block at stop height", func(t *testing.T) {
		sc := NewStopControl(zerolog.Nop(), false)

		// first update is always successful
		oldSet, _, _, err := sc.SetStopHeight(21, false)
		require.NoError(t, err)
		require.False(t, oldSet)

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sc.BlockProcessable(header)

		oldSet, _, _, err = sc.SetStopHeight(37, false)
		require.NoError(t, err)
		require.True(t, oldSet)

		// block at stop height, it should be skipped
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sc.BlockProcessable(header)

		_, _, _, err = sc.SetStopHeight(2137, false)
		require.Error(t, err)
	})

	t.Run("when processing finalized blocks", func(t *testing.T) {

		execState := new(mock.ReadOnlyExecutionState)

		sc := NewStopControl(zerolog.Nop(), false)

		// first update is always successful
		oldSet, _, _, err := sc.SetStopHeight(21, false)
		require.NoError(t, err)
		require.False(t, oldSet)

		// make execution check pretends block has been executed
		execState.On("StateCommitmentByBlockID", mock2.Anything, mock2.Anything).Return(nil, nil)

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sc.BlockFinalized(context.TODO(), execState, header)

		oldSet, _, _, err = sc.SetStopHeight(37, false)
		require.NoError(t, err)
		require.True(t, oldSet)

		// block at stop height, it should be skipped
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sc.BlockFinalized(context.TODO(), execState, header)

		_, _, _, err = sc.SetStopHeight(2137, false)
		require.Error(t, err)
	})
}

// TestOutOfOrderCallsStillWorks check if StopControl behaves properly even if blocks and finalization
// arrive out of order. While proper order should be guaranteed by consensus follower, it's still worth
// a test in case functions are called
func TestOutOfOrderCallsStillWorks(t *testing.T) {

	execState := new(mock.ReadOnlyExecutionState)
	//execState.On()

	headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
	headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
	headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22
	headerD := unittest.BlockHeaderWithParentFixture(headerC) // 23

	sc := NewStopControl(zerolog.Nop(), false)

	// set stop at 22, so 21 is the last height which should be processed
	oldSet, _, _, err := sc.SetStopHeight(22, false)
	require.NoError(t, err)
	require.False(t, oldSet)

	sc.BlockFinalized(context.TODO(), execState, headerD)

}

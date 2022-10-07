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
		sah := NewStopControl(zerolog.Nop(), false)

		// first update is always successful
		oldSet, _, _, err := sah.SetStopHeight(21, false)
		require.NoError(t, err)
		require.False(t, oldSet)

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sah.BlockProcessable(header)

		oldSet, _, _, err = sah.SetStopHeight(37, false)
		require.NoError(t, err)
		require.True(t, oldSet)

		// block at stop height, it should be skipped
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sah.BlockProcessable(header)

		_, _, _, err = sah.SetStopHeight(2137, false)
		require.Error(t, err)
	})

	t.Run("when processing finalized blocks", func(t *testing.T) {

		execState := new(mock.ReadOnlyExecutionState)

		sah := NewStopControl(zerolog.Nop(), false)

		// first update is always successful
		oldSet, _, _, err := sah.SetStopHeight(21, false)
		require.NoError(t, err)
		require.False(t, oldSet)

		// make execution check pretends block has been executed
		execState.On("StateCommitmentByBlockID", mock2.Anything, mock2.Anything).Return(nil, nil)

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sah.BlockFinalized(context.TODO(), execState, header)

		oldSet, _, _, err = sah.SetStopHeight(37, false)
		require.NoError(t, err)
		require.True(t, oldSet)

		// block at stop height, it should be skipped
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sah.BlockFinalized(context.TODO(), execState, header)

		_, _, _, err = sah.SetStopHeight(2137, false)
		require.Error(t, err)
	})

}

package ingestion

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestCannotSetNewValuesAfterStoppingStarted(t *testing.T) {

	sah := NewStopControl(zerolog.Nop(), false)

	// first update is always successful
	oldSet, _, _, err := sah.SetStopHeight(21, false)
	require.NoError(t, err)
	require.False(t, oldSet)

	// no stopping has started yet
	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
	sah.BlockProcessable(header)

	oldSet, _, _, err = sah.SetStopHeight(37, false)
	require.NoError(t, err)
	require.True(t, oldSet)

	// no stopping has started yet
	header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
	sah.BlockProcessable(header)

	_, _, _, err = sah.SetStopHeight(2137, false)
	require.Error(t, err)

}

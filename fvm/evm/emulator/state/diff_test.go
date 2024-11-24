package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateDiff(t *testing.T) {
	offchainState, err := ImportEVMStateFromGob("/var/flow2/evm-state-from-gobs-218215348/")
	require.NoError(t, err)

	enState, err := ImportEVMStateFromGob("/var/flow2/evm-state-from-gobs-218215348/")
	require.NoError(t, err)

	differences := Diff(enState, offchainState)

	require.Len(t, differences, 0)
}

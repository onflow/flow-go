package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateDiff(t *testing.T) {
	enState, err := ImportEVMState("~/Downloads/compare-state/evm-state-from-checkpoint/")
	require.NoError(t, err)

	offchainState, err := ImportEVMState("~/Downloads/compare-state/evm-state-from-offchain/")
	require.NoError(t, err)

	differences := Diff(enState, offchainState)

	require.Len(t, differences, 0)
}

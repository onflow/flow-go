package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateDiff(t *testing.T) {
	offchainState, err := ImportEVMState("/Users/leozhang/Downloads/compare-state/evm-state-from-gobs/")
	require.NoError(t, err)

	enState, err := ImportEVMState("/Users/leozhang/Downloads/compare-state/evm-state-from-checkpoint/")
	require.NoError(t, err)

	differences := Diff(enState, offchainState)

	require.Len(t, differences, 0)
}

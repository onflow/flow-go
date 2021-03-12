package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
)

func TestStateHolder(t *testing.T) {
	view := utils.NewSimpleView()
	st := state.NewState(view)
	sth := state.NewStateHolder(st)

	t.Run("test nesting/rollup functionality", func(t *testing.T) {
		value := createByteArray(1)

		// first child
		sth.Nest()
		require.Equal(t, sth.StartState(), st)
		require.NotEqual(t, sth.State(), st)

		size := uint64(len("address") + len("controller") + len("key1") + len(value))
		err := sth.State().Set("address", "controller", "key1", value)
		require.NoError(t, err)
		require.Equal(t, size, sth.State().TotalBytesWritten)

		// roll up with merge
		err = sth.RollUpWithMerge()
		require.NoError(t, err)
		require.Equal(t, sth.State(), st)
		require.Equal(t, uint64(1), sth.State().WriteCounter)
		require.Equal(t, size, sth.State().TotalBytesWritten)

		// second child
		sth.Nest()
		err = sth.State().Set("address", "controller", "key2", value)
		require.NoError(t, err)
		require.Equal(t, uint64(1), sth.State().WriteCounter)

		// a grandchild
		sth.Nest()
		err = sth.State().Set("address", "controller", "key3", value)
		require.NoError(t, err)
		require.Equal(t, uint64(1), sth.State().WriteCounter)

		// ignore this child
		err = sth.RollUpNoMerge()
		require.NoError(t, err)
		require.Equal(t, uint64(1), sth.State().WriteCounter)
	})
}

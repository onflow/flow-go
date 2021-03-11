package state_test

import (
	"testing"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/stretchr/testify/require"
)

func TestStateManager(t *testing.T) {
	view := utils.NewSimpleView()
	st := state.NewState(view)
	stm := state.NewStateManager(st)

	t.Run("test nesting/rollup functionality", func(t *testing.T) {
		value := createByteArray(1)

		// first child
		stm.Nest()
		require.Equal(t, stm.StartState(), st)
		require.NotEqual(t, stm.State(), st)

		size := uint64(len("address") + len("controller") + len("key1") + len(value))
		err := stm.State().Set("address", "controller", "key1", value)
		require.NoError(t, err)
		require.Equal(t, size, stm.State().TotalBytesWritten)

		// roll up with merge
		err = stm.RollUpWithMerge()
		require.NoError(t, err)
		require.Equal(t, stm.State(), st)
		require.Equal(t, uint64(1), stm.State().WriteCounter)
		require.Equal(t, size, stm.State().TotalBytesWritten)

		// second child
		stm.Nest()
		err = stm.State().Set("address", "controller", "key2", value)
		require.NoError(t, err)
		require.Equal(t, uint64(1), stm.State().WriteCounter)

		// a grandchild
		stm.Nest()
		err = stm.State().Set("address", "controller", "key3", value)
		require.NoError(t, err)
		require.Equal(t, uint64(1), stm.State().WriteCounter)

		// ignore this child
		err = stm.RollUpNoMerge()
		require.NoError(t, err)
		require.Equal(t, uint64(1), stm.State().WriteCounter)
	})
}

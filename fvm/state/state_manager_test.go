package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
)

func TestStateManager(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger)
	stm := state.NewStateManager(st)

	t.Run("test nesting/rollup functionality", func(t *testing.T) {
		value := createByteArray(1)

		// first child
		stm.Nest()
		require.Equal(t, stm.StartState(), st)
		require.NotEqual(t, stm.State(), st)

		err := stm.State().Set("address", "controller", "key1", value)
		require.NoError(t, err)

		// roll up with merge
		err = stm.RollUpWithMerge()
		require.NoError(t, err)
		require.Equal(t, stm.State(), st)
		require.Equal(t, uint64(1), st.ToBeWrittenCounter)
		require.Equal(t, 1, len(st.Touches()))

		// second child
		stm.Nest()
		err = stm.State().Set("address", "controller", "key2", value)
		require.NoError(t, err)
		require.Equal(t, 1, len(stm.State().Touches()))

		// a grandchild
		stm.Nest()
		err = stm.State().Set("address", "controller", "key3", value)
		require.NoError(t, err)
		require.Equal(t, 1, len(stm.State().Touches()))

		// ignore this child
		err = stm.RollUpNoMerge()
		require.NoError(t, err)
		require.Equal(t, 1, len(st.Touches()))
		require.Equal(t, 1, len(stm.State().Touches()))

		err = stm.RollUpWithTouchMergeOnly()
		require.NoError(t, err)
		require.Equal(t, 2, len(st.Touches()))
		require.Equal(t, 2, len(stm.State().Touches()))

	})
}

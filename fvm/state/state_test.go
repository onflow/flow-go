package state_test

import (
	"testing"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/stretchr/testify/require"
)

func TestState_DraftFunctionality(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger)

	value := createByteArray(11)
	err := st.Update("address", "controller", "key", value)
	require.NoError(t, err)

	// read from draft
	v, err := st.Read("address", "controller", "key")
	require.NoError(t, err)
	require.Equal(t, v, value)

	// commit
	st.Commit()
	v, err = st.Read("address", "controller", "key")
	require.NoError(t, err)
	require.Equal(t, v, value)

	value2 := createByteArray(12)
	err = st.Update("address", "controller", "key", value2)
	require.NoError(t, err)

	// read from draft
	v, err = st.Read("address", "controller", "key")
	require.NoError(t, err)
	require.Equal(t, v, value2)

	// rollback
	st.Rollback()
	v, err = st.Read("address", "controller", "key")
	require.NoError(t, err)
	require.Equal(t, v, value)
}

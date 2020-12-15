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
	err = st.Commit()
	require.NoError(t, err)
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

func TestState_MaxValueSize(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger, state.WithMaxValueSizeAllowed(6))

	// update should pass
	value := createByteArray(5)
	err := st.Update("address", "controller", "key", value)
	require.NoError(t, err)

	// update shouldn't pass
	value = createByteArray(7)
	err = st.Update("address", "controller", "key", value)
	require.Error(t, err)
}

func TestState_MaxKeySize(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger, state.WithMaxKeySizeAllowed(6))

	// read
	_, err := st.Read("1", "2", "3")
	require.NoError(t, err)

	// read
	_, err = st.Read("123", "234", "345")
	require.Error(t, err)

	// update
	err = st.Update("1", "2", "3", []byte{})
	require.NoError(t, err)

	// read
	err = st.Update("123", "234", "345", []byte{})
	require.Error(t, err)

}

func TestState_MaxInteraction(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger, state.WithMaxInteractionSizeAllowed(12))

	// read - interaction 3
	_, err := st.Read("1", "2", "3")
	require.Equal(t, st.InteractionUsed(), uint64(3))
	require.NoError(t, err)

	// read - interaction 12
	_, err = st.Read("123", "234", "345")
	require.Equal(t, st.InteractionUsed(), uint64(12))
	require.NoError(t, err)

	// read - interaction 21
	_, err = st.Read("234", "345", "456")
	require.Equal(t, st.InteractionUsed(), uint64(21))
	require.Error(t, err)

	st = state.NewState(ledger, state.WithMaxInteractionSizeAllowed(9))

	// update - 0
	err = st.Update("1", "2", "3", []byte{'A'})
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(0))

	// commit
	err = st.Commit()
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(4))

	// read - interaction 4 (already in read cache)
	_, err = st.Read("1", "2", "3")
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(4))

	// read - interaction 7
	_, err = st.Read("2", "3", "4")
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(7))

	// read - interaction 10
	_, err = st.Read("3", "4", "5")
	require.Error(t, err)
}

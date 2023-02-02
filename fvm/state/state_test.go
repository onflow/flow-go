package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

func createByteArray(size int) []byte {
	bytes := make([]byte, size)
	for i := range bytes {
		bytes[i] = 255
	}
	return bytes
}

func TestState_ChildMergeFunctionality(t *testing.T) {
	view := utils.NewSimpleView()
	st := state.NewState(view, state.DefaultParameters())

	t.Run("test read from parent state (backoff)", func(t *testing.T) {
		key := flow.NewRegisterID("address", "key1")
		value := createByteArray(1)
		// set key1 on parent
		err := st.Set(key, value)
		require.NoError(t, err)

		// read key1 on child
		stChild := st.NewChild()
		v, err := stChild.Get(key)
		require.NoError(t, err)
		require.Equal(t, v, value)
	})

	t.Run("test write to child (no merge)", func(t *testing.T) {
		key := flow.NewRegisterID("address", "key2")
		value := createByteArray(2)
		stChild := st.NewChild()

		// set key2 on child
		err := stChild.Set(key, value)
		require.NoError(t, err)

		// read key2 on parent
		v, err := st.Get(key)
		require.NoError(t, err)
		require.Equal(t, len(v), 0)
	})

	t.Run("test write to child and merge", func(t *testing.T) {
		key := flow.NewRegisterID("address", "key3")
		value := createByteArray(3)
		stChild := st.NewChild()

		// set key3 on child
		err := stChild.Set(key, value)
		require.NoError(t, err)

		// read before merge
		v, err := st.Get(key)
		require.NoError(t, err)
		require.Equal(t, len(v), 0)

		// merge to parent
		err = st.MergeState(stChild)
		require.NoError(t, err)

		// read key3 on parent
		v, err = st.Get(key)
		require.NoError(t, err)
		require.Equal(t, v, value)
	})

	t.Run("test write to ledger", func(t *testing.T) {
		key := flow.NewRegisterID("address", "key4")
		value := createByteArray(4)
		// set key4 on parent
		err := st.Set(key, value)
		require.NoError(t, err)

		// now should be part of the ledger
		v, err := view.Get(key)
		require.NoError(t, err)
		require.Equal(t, v, value)
	})

}

func TestState_MaxValueSize(t *testing.T) {
	view := utils.NewSimpleView()
	st := state.NewState(view, state.DefaultParameters().WithMaxValueSizeAllowed(6))

	key := flow.NewRegisterID("address", "key")

	// update should pass
	value := createByteArray(5)
	err := st.Set(key, value)
	require.NoError(t, err)

	// update shouldn't pass
	value = createByteArray(7)
	err = st.Set(key, value)
	require.Error(t, err)
}

func TestState_MaxKeySize(t *testing.T) {
	view := utils.NewSimpleView()
	st := state.NewState(view, state.DefaultParameters().WithMaxKeySizeAllowed(4))

	key1 := flow.NewRegisterID("1", "2")
	key2 := flow.NewRegisterID("123", "234")

	// read
	_, err := st.Get(key1)
	require.NoError(t, err)

	// read
	_, err = st.Get(key2)
	require.Error(t, err)

	// update
	err = st.Set(key1, []byte{})
	require.NoError(t, err)

	// read
	err = st.Set(key2, []byte{})
	require.Error(t, err)

}

func TestState_MaxInteraction(t *testing.T) {
	view := utils.NewSimpleView()
	st := state.NewState(
		view,
		state.DefaultParameters().
			WithMeterParameters(
				meter.DefaultParameters().WithStorageInteractionLimit(12)))

	key1 := flow.NewRegisterID("1", "2")

	// read - interaction 2
	_, err := st.Get(key1)
	require.Equal(t, st.InteractionUsed(), uint64(2))
	require.NoError(t, err)

	// read - interaction 8
	_, err = st.Get(flow.NewRegisterID("123", "234"))
	require.Equal(t, st.InteractionUsed(), uint64(8))
	require.NoError(t, err)

	// read - interaction 14
	_, err = st.Get(flow.NewRegisterID("234", "345"))
	require.Equal(t, st.InteractionUsed(), uint64(14))
	require.Error(t, err)

	st = state.NewState(
		view,
		state.DefaultParameters().
			WithMeterParameters(
				meter.DefaultParameters().WithStorageInteractionLimit(6)))
	stChild := st.NewChild()

	// update - 0
	err = stChild.Set(key1, []byte{'A'})
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(0))

	// commit
	err = st.MergeState(stChild)
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(3))

	// read - interaction 3 (already in read cache)
	_, err = st.Get(key1)
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(3))

	// read - interaction 5
	_, err = st.Get(flow.NewRegisterID("2", "3"))
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(5))

	// read - interaction 7
	_, err = st.Get(flow.NewRegisterID("3", "4"))
	require.Error(t, err)

}

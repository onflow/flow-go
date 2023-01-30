package state_test

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/onflow/atree"

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
		key := flow.RegisterID{Owner: "address", Key: "key1"}
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
		key := flow.RegisterID{Owner: "address", Key: "key2"}
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
		key := flow.RegisterID{Owner: "address", Key: "key3"}
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
		key := flow.RegisterID{Owner: "address", Key: "key4"}
		value := createByteArray(4)
		// set key4 on parent
		err := st.Set(key, value)
		require.NoError(t, err)

		// now should be part of the ledger
		v, err := view.Get(key.Owner, key.Key)
		require.NoError(t, err)
		require.Equal(t, v, value)
	})

}

func TestState_MaxValueSize(t *testing.T) {
	view := utils.NewSimpleView()
	st := state.NewState(view, state.DefaultParameters().WithMaxValueSizeAllowed(6))

	key := flow.RegisterID{Owner: "address", Key: "key"}

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

	key1 := flow.RegisterID{Owner: "1", Key: "2"}
	key2 := flow.RegisterID{Owner: "123", Key: "234"}

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

	key1 := flow.RegisterID{Owner: "1", Key: "2"}

	// read - interaction 2
	_, err := st.Get(key1)
	require.Equal(t, st.InteractionUsed(), uint64(2))
	require.NoError(t, err)

	// read - interaction 8
	_, err = st.Get(flow.RegisterID{Owner: "123", Key: "234"})
	require.Equal(t, st.InteractionUsed(), uint64(8))
	require.NoError(t, err)

	// read - interaction 14
	_, err = st.Get(flow.RegisterID{Owner: "234", Key: "345"})
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
	_, err = st.Get(flow.RegisterID{Owner: "2", Key: "3"})
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(5))

	// read - interaction 7
	_, err = st.Get(flow.RegisterID{Owner: "3", Key: "4"})
	require.Error(t, err)

}

func TestState_IsFVMStateKey(t *testing.T) {
	require.True(t, state.IsFVMStateKey("", state.UUIDKey))
	require.True(t, state.IsFVMStateKey("", state.AddressStateKey))
	require.False(t, state.IsFVMStateKey("", "other"))
	require.False(t, state.IsFVMStateKey("Address", state.UUIDKey))
	require.False(t, state.IsFVMStateKey("Address", state.AddressStateKey))
	require.True(t, state.IsFVMStateKey("Address", "public_key_12"))
	require.True(t, state.IsFVMStateKey("Address", state.ContractNamesKey))
	require.True(t, state.IsFVMStateKey("Address", "code.MYCODE"))
	require.True(t, state.IsFVMStateKey("Address", state.AccountStatusKey))
	require.False(t, state.IsFVMStateKey("Address", "anything else"))
}

func TestAccounts_PrintableKey(t *testing.T) {
	// slab with 189 should result in \\xbd
	slabIndex := atree.StorageIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 189})
	key := string(atree.SlabIndexToLedgerKey(slabIndex))
	require.False(t, utf8.ValidString(key))
	printable := state.PrintableKey(key)
	require.True(t, utf8.ValidString(printable))
	require.Equal(t, "$189", printable)

	// non slab invalid utf-8
	key = "a\xc5z"
	require.False(t, utf8.ValidString(key))
	printable = state.PrintableKey(key)
	require.True(t, utf8.ValidString(printable))
	require.Equal(t, "#61c57a", printable)
}

package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
)

func TestState_ChildMergeFunctionality(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger)

	t.Run("test read from parent state (backoff)", func(t *testing.T) {
		key := "key1"
		value := createByteArray(1)
		// set key1 on parent
		err := st.Set("address", "controller", key, value)
		require.NoError(t, err)

		// read key1 on child
		stChild := st.NewChild()
		v, err := stChild.Get("address", "controller", key)
		require.NoError(t, err)
		require.Equal(t, v, value)
	})

	t.Run("test write to child (no merge)", func(t *testing.T) {
		key := "key2"
		value := createByteArray(2)
		stChild := st.NewChild()

		// set key2 on child
		err := stChild.Set("address", "controller", key, value)
		require.NoError(t, err)

		// read key2 on parent
		v, err := st.Get("address", "controller", key)
		require.NoError(t, err)
		require.Equal(t, len(v), 0)
	})

	t.Run("test write to child and merge", func(t *testing.T) {
		key := "key3"
		value := createByteArray(3)
		stChild := st.NewChild()

		// set key3 on child
		err := stChild.Set("address", "controller", key, value)
		require.NoError(t, err)

		// merge to parent
		err = st.MergeState(stChild)
		require.NoError(t, err)

		// read key3 on parent
		v, err := st.Get("address", "controller", key)
		require.NoError(t, err)
		require.Equal(t, v, value)
	})

	t.Run("test write to ledger", func(t *testing.T) {
		key := "key4"
		value := createByteArray(4)
		// set key4 on parent
		err := st.Set("address", "controller", key, value)
		require.NoError(t, err)

		// shouldn't be part of the ledger before applyToLedger call
		v, err := ledger.Get("address", "controller", key)
		require.NoError(t, err)
		require.Equal(t, len(v), 0)

		// now should be part of the ledger
		err = st.ApplyDeltaToLedger()
		require.NoError(t, err)
		v, err = ledger.Get("address", "controller", key)
		require.NoError(t, err)
		require.Equal(t, v, value)
	})

}

func TestState_InteractionMeasuring(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger)

	key := "key1"
	value := createByteArray(1)
	err := st.Set("address", "controller", key, value)
	require.NoError(t, err)
	require.Equal(t, uint64(0), st.ReadCounter)
	require.Equal(t, uint64(0), st.WriteCounter)
	require.Equal(t, uint64(1), st.ToBeWrittenCounter)
	require.Equal(t, uint64(0), st.TotalBytesRead)
	require.Equal(t,
		uint64(len("address")+len("controller")+len(key)+len(value)),
		st.TotalBytesToBeWritten)
	require.Equal(t, uint64(0), st.TotalBytesWritten)
	require.Equal(t, uint64(0), st.TotalBytesRead)
	require.Equal(t, len(st.Touches()), 1)

	// should read from the delta
	// addition to the touch but
	// should not impact totalBytesRead
	v, err := st.Get("address", "controller", key)
	require.NoError(t, err)
	require.Equal(t, v, value)
	require.Equal(t, len(st.Touches()), 2)
	require.Equal(t, uint64(0), st.TotalBytesRead)

	// non existing key
	// should be appended to touches
	// should be counted towards reading from the ledger
	key2 := "key2"
	_, err = st.Get("address", "controller", key2)
	require.NoError(t, err)
	require.Equal(t, len(st.Touches()), 3)
	require.Equal(t,
		uint64(len("address")+len("controller")+len(key)),
		st.TotalBytesRead)

	// read again should be from cache
	// but not an addition ot the ledger read
	_, err = st.Get("address", "controller", key2)
	require.NoError(t, err)
	require.Equal(t, len(st.Touches()), 4)
	require.Equal(t,
		uint64(len("address")+len("controller")+len(key)),
		st.TotalBytesRead)

	// apply to ledger, totalBytesWritten should be updated now
	err = st.ApplyDeltaToLedger()
	require.NoError(t, err)
	require.Equal(t,
		uint64(len("address")+len("controller")+len(key)+len(value)),
		st.TotalBytesWritten)
}

func TestState_MaxValueSize(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger, state.WithMaxValueSizeAllowed(6))

	// update should pass
	value := createByteArray(5)
	err := st.Set("address", "controller", "key", value)
	require.NoError(t, err)

	// update shouldn't pass
	value = createByteArray(7)
	err = st.Set("address", "controller", "key", value)
	require.Error(t, err)
}

func TestState_MaxKeySize(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger, state.WithMaxKeySizeAllowed(6))

	// read
	_, err := st.Get("1", "2", "3")
	require.NoError(t, err)

	// read
	_, err = st.Get("123", "234", "345")
	require.Error(t, err)

	// update
	err = st.Set("1", "2", "3", []byte{})
	require.NoError(t, err)

	// read
	err = st.Set("123", "234", "345", []byte{})
	require.Error(t, err)

}

func TestState_MaxInteraction(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger, state.WithMaxInteractionSizeAllowed(12))

	// read - interaction 3
	_, err := st.Get("1", "2", "3")
	require.Equal(t, st.InteractionUsed(), uint64(3))
	require.NoError(t, err)

	// read - interaction 12
	_, err = st.Get("123", "234", "345")
	require.Equal(t, st.InteractionUsed(), uint64(12))
	require.NoError(t, err)

	// read - interaction 21
	_, err = st.Get("234", "345", "456")
	require.Equal(t, st.InteractionUsed(), uint64(21))
	require.Error(t, err)

	st = state.NewState(ledger, state.WithMaxInteractionSizeAllowed(9))
	stChild := st.NewChild()

	// update - 0
	err = stChild.Set("1", "2", "3", []byte{'A'})
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(0))

	// commit
	err = st.MergeState(stChild)
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(0))

	// apply delta to ledger
	err = st.ApplyDeltaToLedger()
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(4))

	// read - interaction 4 (already in read cache)
	_, err = st.Get("1", "2", "3")
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(4))

	// read - interaction 7
	_, err = st.Get("2", "3", "4")
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(7))

	// read - interaction 10
	_, err = st.Get("3", "4", "5")
	require.Error(t, err)
}

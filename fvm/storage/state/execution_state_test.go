package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func createByteArray(size int) []byte {
	bytes := make([]byte, size)
	for i := range bytes {
		bytes[i] = 255
	}
	return bytes
}

func TestExecutionState_Finalize(t *testing.T) {
	parent := state.NewExecutionState(nil, state.DefaultParameters())

	child := parent.NewChild()

	readId := flow.NewRegisterID(unittest.RandomAddressFixture(), "x")

	_, err := child.Get(readId)
	require.NoError(t, err)

	writeId := flow.NewRegisterID(unittest.RandomAddressFixture(), "y")
	writeValue := flow.RegisterValue("a")

	err = child.Set(writeId, writeValue)
	require.NoError(t, err)

	childSnapshot := child.Finalize()

	require.Equal(
		t,
		map[flow.RegisterID]struct{}{
			readId: {},
		},
		childSnapshot.ReadSet)

	require.Equal(
		t,
		map[flow.RegisterID]flow.RegisterValue{
			writeId: writeValue,
		},
		childSnapshot.WriteSet)

	require.NotNil(t, childSnapshot.SpockSecret)
	require.NotNil(t, childSnapshot.Meter)

	parentSnapshot := parent.Finalize()
	// empty read / write set since child was not merged.
	require.Empty(t, parentSnapshot.ReadSet)
	require.Empty(t, parentSnapshot.WriteSet)
	require.NotNil(t, parentSnapshot.SpockSecret)
	require.NotNil(t, parentSnapshot.Meter)

}

func TestExecutionState_ChildMergeFunctionality(t *testing.T) {
	st := state.NewExecutionState(nil, state.DefaultParameters())

	owner := unittest.RandomAddressFixture()

	t.Run("test read from parent state (backoff)", func(t *testing.T) {
		key := flow.NewRegisterID(owner, "key1")
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
		key := flow.NewRegisterID(owner, "key2")
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
		key := flow.NewRegisterID(owner, "key3")
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
		err = st.Merge(stChild.Finalize())
		require.NoError(t, err)

		// read key3 on parent
		v, err = st.Get(key)
		require.NoError(t, err)
		require.Equal(t, v, value)
	})

	t.Run("test write to ledger", func(t *testing.T) {
		key := flow.NewRegisterID(owner, "key4")
		value := createByteArray(4)
		// set key4 on parent
		err := st.Set(key, value)
		require.NoError(t, err)

		// now should be part of the ledger
		v, err := st.Get(key)
		require.NoError(t, err)
		require.Equal(t, v, value)
	})

}

func TestExecutionState_MaxValueSize(t *testing.T) {
	st := state.NewExecutionState(
		nil,
		state.DefaultParameters().WithMaxValueSizeAllowed(6))

	key := flow.NewRegisterID(unittest.RandomAddressFixture(), "key")

	// update should pass
	value := createByteArray(5)
	err := st.Set(key, value)
	require.NoError(t, err)

	// update shouldn't pass
	value = createByteArray(7)
	err = st.Set(key, value)
	require.Error(t, err)
}

func TestExecutionState_MaxKeySize(t *testing.T) {
	st := state.NewExecutionState(
		nil,
		// Note: owners are always 8 bytes
		state.DefaultParameters().WithMaxKeySizeAllowed(8+2))

	key1 := flow.NewRegisterID(unittest.RandomAddressFixture(), "23")
	key2 := flow.NewRegisterID(unittest.RandomAddressFixture(), "234")

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

func TestExecutionState_MaxInteraction(t *testing.T) {
	key1 := flow.NewRegisterID(unittest.RandomAddressFixture(), "2")
	key1Size := uint64(8 + 1)

	value1 := []byte("A")
	value1Size := uint64(1)

	key2 := flow.NewRegisterID(unittest.RandomAddressFixture(), "23")
	key2Size := uint64(8 + 2)

	key3 := flow.NewRegisterID(unittest.RandomAddressFixture(), "345")
	key3Size := uint64(8 + 3)

	key4 := flow.NewRegisterID(unittest.RandomAddressFixture(), "4567")
	key4Size := uint64(8 + 4)

	st := state.NewExecutionState(
		nil,
		state.DefaultParameters().
			WithMeterParameters(
				meter.DefaultParameters().WithStorageInteractionLimit(
					key1Size+key2Size+key3Size-1)))

	// read - interaction 2
	_, err := st.Get(key1)
	require.Equal(t, st.InteractionUsed(), key1Size)
	require.NoError(t, err)

	// read - interaction 8
	_, err = st.Get(key2)
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), key1Size+key2Size)

	// read - interaction 14
	_, err = st.Get(key3)
	require.Error(t, err)
	require.Equal(t, st.InteractionUsed(), key1Size+key2Size+key3Size)

	st = state.NewExecutionState(
		nil,
		state.DefaultParameters().
			WithMeterParameters(
				meter.DefaultParameters().WithStorageInteractionLimit(
					key1Size+value1Size+key2Size)))
	stChild := st.NewChild()

	// update - 0
	err = stChild.Set(key1, value1)
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), uint64(0))

	// commit
	err = st.Merge(stChild.Finalize())
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), key1Size+value1Size)

	// read - interaction 3 (already in read cache)
	_, err = st.Get(key1)
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), key1Size+value1Size)

	// read - interaction 5
	_, err = st.Get(key2)
	require.NoError(t, err)
	require.Equal(t, st.InteractionUsed(), key1Size+value1Size+key2Size)

	// read - interaction 7
	_, err = st.Get(key4)
	require.Error(t, err)
	require.Equal(
		t,
		st.InteractionUsed(),
		key1Size+value1Size+key2Size+key4Size)
}

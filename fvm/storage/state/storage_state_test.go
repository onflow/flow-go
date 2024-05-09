package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestStorageStateSet(t *testing.T) {
	fooOwner := unittest.RandomAddressFixture()

	registerId1 := flow.NewRegisterID(fooOwner, "1")
	value1 := flow.RegisterValue([]byte("value1"))

	registerId2 := flow.NewRegisterID(fooOwner, "2")
	value2 := flow.RegisterValue([]byte("value2"))

	state := newStorageState(nil)

	err := state.Set(registerId1, []byte("old value"))
	require.NoError(t, err)

	err = state.Set(registerId2, value2)
	require.NoError(t, err)

	err = state.Set(registerId1, value1)
	require.NoError(t, err)

	snapshot := state.Finalize()
	require.Empty(t, snapshot.ReadSet)
	require.Equal(
		t,
		snapshot.WriteSet,
		map[flow.RegisterID]flow.RegisterValue{
			registerId1: value1,
			registerId2: value2,
		})
}

func TestStorageStateGetFromNilBase(t *testing.T) {
	state := newStorageState(nil)
	value, err := state.Get(flow.NewRegisterID(unittest.RandomAddressFixture(), "bar"))
	require.NoError(t, err)
	require.Nil(t, value)
}

func TestStorageStateGetFromBase(t *testing.T) {
	registerId := flow.NewRegisterID(flow.EmptyAddress, "base")
	baseValue := flow.RegisterValue([]byte("base"))

	state := newStorageState(
		snapshot.MapStorageSnapshot{
			registerId: baseValue,
		})

	value, err := state.Get(registerId)
	require.NoError(t, err)
	require.Equal(t, value, baseValue)

	// Finalize to ensure read set is updated.
	snapshot := state.Finalize()
	require.Equal(
		t,
		snapshot.ReadSet,
		map[flow.RegisterID]struct{}{
			registerId: struct{}{},
		})
	require.Empty(t, snapshot.WriteSet)

	// Override a previous read value won't change the read set.
	updatedValue := flow.RegisterValue([]byte("value"))
	err = state.Set(registerId, updatedValue)
	require.NoError(t, err)

	snapshot = state.Finalize()
	require.Equal(
		t,
		snapshot.ReadSet,
		map[flow.RegisterID]struct{}{
			registerId: struct{}{},
		})
	require.Equal(
		t,
		snapshot.WriteSet,
		map[flow.RegisterID]flow.RegisterValue{
			registerId: updatedValue,
		})
}

func TestStorageStateGetFromWriteSet(t *testing.T) {
	registerId := flow.NewRegisterID(flow.EmptyAddress, "base")
	expectedValue := flow.RegisterValue([]byte("base"))

	state := newStorageState(nil)

	err := state.Set(registerId, expectedValue)
	require.NoError(t, err)

	value, err := state.Get(registerId)
	require.NoError(t, err)
	require.Equal(t, value, expectedValue)

	snapshot := state.Finalize()
	require.Empty(t, snapshot.ReadSet)
	require.Equal(
		t,
		snapshot.WriteSet,
		map[flow.RegisterID]flow.RegisterValue{
			registerId: expectedValue,
		})
}

func TestStorageStateMerge(t *testing.T) {
	parentOwner := unittest.RandomAddressFixture()
	childOwner := unittest.RandomAddressFixture()

	baseRegisterId := flow.NewRegisterID(flow.EmptyAddress, "base")
	baseValue := flow.RegisterValue([]byte("base"))

	parentRegisterId1 := flow.NewRegisterID(parentOwner, "1")
	parentValue := flow.RegisterValue([]byte("parent"))

	parentRegisterId2 := flow.NewRegisterID(parentOwner, "2")

	parentRegisterId3 := flow.NewRegisterID(parentOwner, "3")
	originalParentValue3 := flow.RegisterValue([]byte("parent value"))
	updatedParentValue3 := flow.RegisterValue([]byte("child value"))

	childRegisterId1 := flow.NewRegisterID(childOwner, "1")
	childValue1 := flow.RegisterValue([]byte("child"))

	childRegisterId2 := flow.NewRegisterID(childOwner, "2")

	parent := newStorageState(
		snapshot.MapStorageSnapshot{
			baseRegisterId: baseValue,
		})

	err := parent.Set(parentRegisterId1, parentValue)
	require.NoError(t, err)

	value, err := parent.Get(parentRegisterId2)
	require.NoError(t, err)
	require.Nil(t, value)

	err = parent.Set(parentRegisterId3, originalParentValue3)
	require.NoError(t, err)

	child := parent.NewChild()

	err = child.Set(parentRegisterId3, updatedParentValue3)
	require.NoError(t, err)

	value, err = child.Get(baseRegisterId)
	require.NoError(t, err)
	require.Equal(t, value, baseValue)

	value, err = child.Get(parentRegisterId1)
	require.NoError(t, err)
	require.Equal(t, value, parentValue)

	value, err = child.Get(childRegisterId2)
	require.NoError(t, err)
	require.Nil(t, value)

	err = child.Set(childRegisterId1, childValue1)
	require.NoError(t, err)

	childSnapshot := child.Finalize()
	require.Equal(
		t,
		childSnapshot.ReadSet,
		map[flow.RegisterID]struct{}{
			baseRegisterId:    struct{}{},
			parentRegisterId1: struct{}{},
			childRegisterId2:  struct{}{},
		})

	require.Equal(
		t,
		childSnapshot.WriteSet,
		map[flow.RegisterID]flow.RegisterValue{
			childRegisterId1:  childValue1,
			parentRegisterId3: updatedParentValue3,
		})

	// Finalize parent without merging child to see if they are independent.
	parentSnapshot := parent.Finalize()
	require.Equal(
		t,
		parentSnapshot.ReadSet,
		map[flow.RegisterID]struct{}{
			parentRegisterId2: struct{}{},
		})

	require.Equal(
		t,
		parentSnapshot.WriteSet,
		map[flow.RegisterID]flow.RegisterValue{
			parentRegisterId1: parentValue,
			parentRegisterId3: originalParentValue3,
		})

	// Merge the child snapshot and check again
	err = parent.Merge(childSnapshot)
	require.NoError(t, err)

	parentSnapshot = parent.Finalize()
	require.Equal(
		t,
		parentSnapshot.ReadSet,
		map[flow.RegisterID]struct{}{
			// from parent's state
			parentRegisterId2: struct{}{},

			// from child's state (parentRegisterId1 is not included since
			// that value is read from the write set)
			baseRegisterId:   struct{}{},
			childRegisterId2: struct{}{},
		})

	require.Equal(
		t,
		parentSnapshot.WriteSet,
		map[flow.RegisterID]flow.RegisterValue{
			// from parent's state (parentRegisterId3 is overwritten by child)
			parentRegisterId1: parentValue,

			// from parent's state
			childRegisterId1:  childValue1,
			parentRegisterId3: updatedParentValue3,
		})
}

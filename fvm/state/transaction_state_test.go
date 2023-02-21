package state_test

import (
	"math"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func newTestTransactionState() state.NestedTransaction {
	return state.NewTransactionState(
		delta.NewDeltaView(nil),
		state.DefaultParameters(),
	)
}

func TestUnrestrictedNestedTransactionBasic(t *testing.T) {
	txn := newTestTransactionState()

	mainState := txn.MainTransactionId().StateForTestingOnly()

	require.Equal(t, 0, txn.NumNestedTransactions())
	require.False(t, txn.IsParseRestricted())

	id1, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.False(t, txn.IsParseRestricted())

	require.True(t, txn.IsCurrent(id1))

	nestedState1 := id1.StateForTestingOnly()

	id2, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	require.Equal(t, 2, txn.NumNestedTransactions())
	require.False(t, txn.IsParseRestricted())

	require.False(t, txn.IsCurrent(id1))
	require.True(t, txn.IsCurrent(id2))

	nestedState2 := id2.StateForTestingOnly()

	// Ensure the values are written to the correctly nested state

	key := flow.NewRegisterID("address", "key")
	val := createByteArray(2)

	err = txn.Set(key, val)
	require.NoError(t, err)

	v, err := nestedState2.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = nestedState1.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = mainState.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	// Ensure nested transactions are merged correctly

	_, err = txn.CommitNestedTransaction(id2)
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id1))

	v, err = nestedState1.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = mainState.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	_, err = txn.CommitNestedTransaction(id1)
	require.NoError(t, err)

	require.Equal(t, 0, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(txn.MainTransactionId()))

	v, err = mainState.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, v)
}

func TestUnrestrictedNestedTransactionDifferentMeterParams(t *testing.T) {
	txn := newTestTransactionState()

	mainState := txn.MainTransactionId().StateForTestingOnly()

	require.Equal(t, uint(math.MaxUint), mainState.TotalMemoryLimit())

	id1, err := txn.BeginNestedTransactionWithMeterParams(
		meter.DefaultParameters().WithMemoryLimit(1))
	require.NoError(t, err)

	nestedState1 := id1.StateForTestingOnly()

	require.Equal(t, uint(1), nestedState1.TotalMemoryLimit())

	id2, err := txn.BeginNestedTransactionWithMeterParams(
		meter.DefaultParameters().WithMemoryLimit(2))
	require.NoError(t, err)

	nestedState2 := id2.StateForTestingOnly()

	require.Equal(t, uint(2), nestedState2.TotalMemoryLimit())

	// inherits memory limit from parent

	id3, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	nestedState3 := id3.StateForTestingOnly()

	require.Equal(t, uint(2), nestedState3.TotalMemoryLimit())
}

func TestParseRestrictedNestedTransactionBasic(t *testing.T) {
	txn := newTestTransactionState()

	mainId := txn.MainTransactionId()
	mainState := mainId.StateForTestingOnly()

	require.Equal(t, 0, txn.NumNestedTransactions())
	require.False(t, txn.IsParseRestricted())

	id1, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.False(t, txn.IsParseRestricted())

	nestedState := id1.StateForTestingOnly()

	loc1 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{1, 1, 1}),
		Name:    "loc1",
	}

	restrictedId1, err := txn.BeginParseRestrictedNestedTransaction(loc1)
	require.NoError(t, err)

	require.Equal(t, 2, txn.NumNestedTransactions())
	require.True(t, txn.IsParseRestricted())

	restrictedNestedState1 := restrictedId1.StateForTestingOnly()

	loc2 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 2, 2}),
		Name:    "loc2",
	}

	restrictedId2, err := txn.BeginParseRestrictedNestedTransaction(loc2)
	require.NoError(t, err)

	require.Equal(t, 3, txn.NumNestedTransactions())
	require.True(t, txn.IsParseRestricted())

	restrictedNestedState2 := restrictedId2.StateForTestingOnly()

	// Sanity check

	key := flow.NewRegisterID("address", "key")

	v, err := restrictedNestedState2.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = restrictedNestedState1.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = nestedState.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = mainState.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	// Ensures attaching and committing cached nested transaction works

	val := createByteArray(2)

	cachedState := state.NewState(
		delta.NewDeltaView(nil),
		state.DefaultParameters(),
	)

	err = cachedState.Set(key, val)
	require.NoError(t, err)

	err = txn.AttachAndCommitNestedTransaction(cachedState)
	require.NoError(t, err)

	require.Equal(t, 3, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(restrictedId2))

	v, err = restrictedNestedState2.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = restrictedNestedState1.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = nestedState.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = mainState.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	// Ensure nested transactions are merged correctly

	state, err := txn.CommitParseRestrictedNestedTransaction(loc2)
	require.NoError(t, err)
	require.Equal(t, restrictedNestedState2, state)

	require.Equal(t, 2, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(restrictedId1))

	v, err = restrictedNestedState1.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = nestedState.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = mainState.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	state, err = txn.CommitParseRestrictedNestedTransaction(loc1)
	require.NoError(t, err)
	require.Equal(t, restrictedNestedState1, state)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id1))

	v, err = nestedState.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = mainState.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)

	_, err = txn.CommitNestedTransaction(id1)
	require.NoError(t, err)

	require.Equal(t, 0, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(mainId))

	v, err = mainState.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, v)
}

func TestRestartNestedTransaction(t *testing.T) {
	txn := newTestTransactionState()

	require.Equal(t, 0, txn.NumNestedTransactions())

	id, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	key := flow.NewRegisterID("address", "key")
	val := createByteArray(2)

	for i := 0; i < 10; i++ {
		_, err := txn.BeginNestedTransaction()
		require.NoError(t, err)

		err = txn.Set(key, val)
		require.NoError(t, err)
	}

	loc := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{1, 1, 1}),
		Name:    "loc",
	}

	for i := 0; i < 5; i++ {
		_, err := txn.BeginParseRestrictedNestedTransaction(loc)
		require.NoError(t, err)

		err = txn.Set(key, val)
		require.NoError(t, err)
	}

	require.Equal(t, 16, txn.NumNestedTransactions())

	state := id.StateForTestingOnly()
	require.Equal(t, uint64(0), state.InteractionUsed())

	// Restart will merge the meter stat, but not the view delta

	err = txn.RestartNestedTransaction(id)
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id))

	require.Greater(t, state.InteractionUsed(), uint64(0))

	v, err := state.Get(key)
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestRestartNestedTransactionWithInvalidId(t *testing.T) {
	txn := newTestTransactionState()

	require.Equal(t, 0, txn.NumNestedTransactions())

	id, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	key := flow.NewRegisterID("address", "key")
	val := createByteArray(2)

	err = txn.Set(key, val)
	require.NoError(t, err)

	var otherId state.NestedTransactionId
	for i := 0; i < 10; i++ {
		otherId, err = txn.BeginNestedTransaction()
		require.NoError(t, err)

		_, err = txn.CommitNestedTransaction(otherId)
		require.NoError(t, err)
	}

	require.True(t, txn.IsCurrent(id))

	err = txn.RestartNestedTransaction(otherId)
	require.Error(t, err)

	require.True(t, txn.IsCurrent(id))

	v, err := txn.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, v)
}

func TestUnrestrictedCannotCommitParseRestrictedNestedTransaction(t *testing.T) {
	txn := newTestTransactionState()

	loc := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{1, 1, 1}),
		Name:    "loc",
	}

	id, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.False(t, txn.IsParseRestricted())

	_, err = txn.CommitParseRestrictedNestedTransaction(loc)
	require.Error(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id))
}

func TestUnrestrictedCannotCommitMainTransaction(t *testing.T) {
	txn := newTestTransactionState()

	id1, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	id2, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	require.Equal(t, 2, txn.NumNestedTransactions())

	_, err = txn.CommitNestedTransaction(id1)
	require.Error(t, err)

	require.Equal(t, 2, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id2))
}

func TestUnrestrictedCannotCommitUnexpectedNested(t *testing.T) {
	txn := newTestTransactionState()

	mainId := txn.MainTransactionId()

	require.Equal(t, 0, txn.NumNestedTransactions())

	_, err := txn.CommitNestedTransaction(mainId)
	require.Error(t, err)

	require.Equal(t, 0, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(mainId))
}

func TestParseRestrictedCannotBeginUnrestrictedNestedTransaction(t *testing.T) {
	txn := newTestTransactionState()

	loc := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{1, 1, 1}),
		Name:    "loc",
	}

	id1, err := txn.BeginParseRestrictedNestedTransaction(loc)
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())

	id2, err := txn.BeginNestedTransaction()
	require.Error(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id1))
	require.False(t, txn.IsCurrent(id2))
}

func TestParseRestrictedCannotCommitUnrestricted(t *testing.T) {
	txn := newTestTransactionState()

	loc := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{1, 1, 1}),
		Name:    "loc",
	}

	id, err := txn.BeginParseRestrictedNestedTransaction(loc)
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())

	_, err = txn.CommitNestedTransaction(id)
	require.Error(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id))
}

func TestParseRestrictedCannotCommitLocationMismatch(t *testing.T) {
	txn := newTestTransactionState()

	loc := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{1, 1, 1}),
		Name:    "loc",
	}

	id, err := txn.BeginParseRestrictedNestedTransaction(loc)
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())

	other := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{1, 1, 1}),
		Name:    "other",
	}

	cacheableState, err := txn.CommitParseRestrictedNestedTransaction(other)
	require.Error(t, err)
	require.Nil(t, cacheableState)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id))
}

func TestPauseAndResume(t *testing.T) {
	txn := newTestTransactionState()

	key1 := flow.NewRegisterID("addr", "key")
	key2 := flow.NewRegisterID("addr2", "key2")

	val, err := txn.Get(key1)
	require.NoError(t, err)
	require.Nil(t, val)

	id1, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	err = txn.Set(key1, createByteArray(2))
	require.NoError(t, err)

	val, err = txn.Get(key1)
	require.NoError(t, err)
	require.NotNil(t, val)

	pausedState, err := txn.PauseNestedTransaction(id1)
	require.NoError(t, err)

	val, err = txn.Get(key1)
	require.NoError(t, err)
	require.Nil(t, val)

	txn.ResumeNestedTransaction(pausedState)

	val, err = txn.Get(key1)
	require.NoError(t, err)
	require.NotNil(t, val)

	err = txn.Set(key2, createByteArray(2))
	require.NoError(t, err)

	_, err = txn.CommitNestedTransaction(id1)
	require.NoError(t, err)

	val, err = txn.Get(key2)
	require.NoError(t, err)
	require.NotNil(t, val)
}

func TestInvalidCommittedStateModification(t *testing.T) {
	txn := newTestTransactionState()

	id1, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	key := flow.NewRegisterID("addr", "key")
	err = txn.Set(key, createByteArray(2))
	require.NoError(t, err)

	_, err = txn.Get(key)
	require.NoError(t, err)

	committedState, err := txn.CommitNestedTransaction(id1)
	require.NoError(t, err)

	err = committedState.MergeState(
		state.NewState(
			delta.NewDeltaView(nil),
			state.DefaultParameters()))
	require.ErrorContains(t, err, "cannot MergeState on a committed state")

	txn.ResumeNestedTransaction(committedState)

	err = txn.Set(key, createByteArray(2))
	require.ErrorContains(t, err, "cannot Set on a committed state")

	_, err = txn.Get(key)
	require.ErrorContains(t, err, "cannot Get on a committed state")

	_, err = txn.CommitNestedTransaction(id1)
	require.NoError(t, err)
}

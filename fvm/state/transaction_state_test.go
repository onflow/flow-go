package state_test

import (
	"math"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
)

func newTestTransactionState() *state.TransactionState {
	return state.NewTransactionState(
		utils.NewSimpleView(),
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

	addr := "address"
	key := "key"
	val := createByteArray(2)

	err = txn.Set(addr, key, val, true)
	require.NoError(t, err)

	v, err := nestedState2.Get(addr, key, true)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = nestedState1.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = mainState.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	// Ensure nested transactions are merged correctly

	_, err = txn.Commit(id2)
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id1))

	v, err = nestedState1.Get(addr, key, true)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = mainState.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	_, err = txn.Commit(id1)
	require.NoError(t, err)

	require.Equal(t, 0, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(txn.MainTransactionId()))

	v, err = mainState.Get(addr, key, true)
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

	addr := "address"
	key := "key"

	v, err := restrictedNestedState2.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = restrictedNestedState1.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = nestedState.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = mainState.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	// Ensures attaching and committing cached nested transaction works

	val := createByteArray(2)

	cachedState := state.NewState(
		utils.NewSimpleView(),
		state.DefaultParameters(),
	)

	err = cachedState.Set(addr, key, val, true)
	require.NoError(t, err)

	err = txn.AttachAndCommit(cachedState)
	require.NoError(t, err)

	require.Equal(t, 3, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(restrictedId2))

	v, err = restrictedNestedState2.Get(addr, key, true)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = restrictedNestedState1.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = nestedState.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = mainState.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	// Ensure nested transactions are merged correctly

	state, err := txn.CommitParseRestricted(loc2)
	require.NoError(t, err)
	require.Equal(t, restrictedNestedState2, state)

	require.Equal(t, 2, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(restrictedId1))

	v, err = restrictedNestedState1.Get(addr, key, true)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = nestedState.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = mainState.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	state, err = txn.CommitParseRestricted(loc1)
	require.NoError(t, err)
	require.Equal(t, restrictedNestedState1, state)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id1))

	v, err = nestedState.Get(addr, key, true)
	require.NoError(t, err)
	require.Equal(t, val, v)

	v, err = mainState.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)

	_, err = txn.Commit(id1)
	require.NoError(t, err)

	require.Equal(t, 0, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(mainId))

	v, err = mainState.Get(addr, key, true)
	require.NoError(t, err)
	require.Equal(t, val, v)
}

func TestRestartNestedTransaction(t *testing.T) {
	txn := newTestTransactionState()

	require.Equal(t, 0, txn.NumNestedTransactions())

	id, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	addr := "address"
	key := "key"
	val := createByteArray(2)

	for i := 0; i < 10; i++ {
		_, err := txn.BeginNestedTransaction()
		require.NoError(t, err)

		err = txn.Set(addr, key, val, true)
		require.NoError(t, err)
	}

	loc := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{1, 1, 1}),
		Name:    "loc",
	}

	for i := 0; i < 5; i++ {
		_, err := txn.BeginParseRestrictedNestedTransaction(loc)
		require.NoError(t, err)

		err = txn.Set(addr, key, val, true)
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

	v, err := state.Get(addr, key, true)
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestRestartNestedTransactionWithInvalidId(t *testing.T) {
	txn := newTestTransactionState()

	require.Equal(t, 0, txn.NumNestedTransactions())

	id, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	addr := "address"
	key := "key"
	val := createByteArray(2)

	err = txn.Set(addr, key, val, true)
	require.NoError(t, err)

	var otherId state.NestedTransactionId
	for i := 0; i < 10; i++ {
		otherId, err = txn.BeginNestedTransaction()
		require.NoError(t, err)

		_, err = txn.Commit(otherId)
		require.NoError(t, err)
	}

	require.True(t, txn.IsCurrent(id))

	err = txn.RestartNestedTransaction(otherId)
	require.Error(t, err)

	require.True(t, txn.IsCurrent(id))

	v, err := txn.Get(addr, key, true)
	require.NoError(t, err)
	require.Equal(t, val, v)
}

func TestUnrestrictedCannotCommitParseRestricted(t *testing.T) {
	txn := newTestTransactionState()

	loc := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{1, 1, 1}),
		Name:    "loc",
	}

	id, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.False(t, txn.IsParseRestricted())

	_, err = txn.CommitParseRestricted(loc)
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

	_, err = txn.Commit(id1)
	require.Error(t, err)

	require.Equal(t, 2, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id2))
}

func TestUnrestrictedCannotCommitUnexpectedNested(t *testing.T) {
	txn := newTestTransactionState()

	mainId := txn.MainTransactionId()

	require.Equal(t, 0, txn.NumNestedTransactions())

	_, err := txn.Commit(mainId)
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

	_, err = txn.Commit(id)
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

	cacheableState, err := txn.CommitParseRestricted(other)
	require.Error(t, err)
	require.Nil(t, cacheableState)

	require.Equal(t, 1, txn.NumNestedTransactions())
	require.True(t, txn.IsCurrent(id))
}

func TestPauseAndResume(t *testing.T) {
	txn := newTestTransactionState()

	val, err := txn.Get("addr", "key", true)
	require.NoError(t, err)
	require.Nil(t, val)

	id1, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	err = txn.Set("addr", "key", createByteArray(2), true)
	require.NoError(t, err)

	val, err = txn.Get("addr", "key", true)
	require.NoError(t, err)
	require.NotNil(t, val)

	pausedState, err := txn.Pause(id1)
	require.NoError(t, err)

	val, err = txn.Get("addr", "key", true)
	require.NoError(t, err)
	require.Nil(t, val)

	txn.Resume(pausedState)

	val, err = txn.Get("addr", "key", true)
	require.NoError(t, err)
	require.NotNil(t, val)

	err = txn.Set("addr2", "key2", createByteArray(2), true)
	require.NoError(t, err)

	_, err = txn.Commit(id1)
	require.NoError(t, err)

	val, err = txn.Get("addr2", "key2", true)
	require.NoError(t, err)
	require.NotNil(t, val)
}

func TestInvalidCommittedStateModification(t *testing.T) {
	txn := newTestTransactionState()

	id1, err := txn.BeginNestedTransaction()
	require.NoError(t, err)

	err = txn.Set("addr", "key", createByteArray(2), true)
	require.NoError(t, err)

	_, err = txn.Get("addr", "key", true)
	require.NoError(t, err)

	committedState, err := txn.Commit(id1)
	require.NoError(t, err)

	err = committedState.MergeState(
		state.NewState(utils.NewSimpleView(), state.DefaultParameters()))
	require.ErrorContains(t, err, "cannot MergeState on a committed state")

	txn.Resume(committedState)

	err = txn.Set("addr", "key", createByteArray(2), true)
	require.ErrorContains(t, err, "cannot Set on a committed state")

	_, err = txn.Get("addr", "key", true)
	require.ErrorContains(t, err, "cannot Get on a committed state")

	_, err = txn.Commit(id1)
	require.NoError(t, err)
}

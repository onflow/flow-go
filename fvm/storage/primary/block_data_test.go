package primary

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/errors"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
)

func TestBlockDataWithTransactionOffset(t *testing.T) {
	key := flow.RegisterID{
		Owner: "",
		Key:   "key",
	}
	expectedValue := flow.RegisterValue([]byte("value"))

	snapshotTime := logical.Time(18)

	block := NewBlockData(
		snapshot.MapStorageSnapshot{
			key: expectedValue,
		},
		snapshotTime)

	snapshot := block.LatestSnapshot()
	require.Equal(t, snapshotTime, snapshot.SnapshotTime())

	value, err := snapshot.Get(key)
	require.NoError(t, err)
	require.Equal(t, expectedValue, value)
}

func TestBlockDataNormalTransactionInvalidExecutionTime(t *testing.T) {
	snapshotTime := logical.Time(5)
	block := NewBlockData(nil, snapshotTime)

	txn, err := block.NewTransactionData(-1, state.DefaultParameters())
	require.ErrorContains(t, err, "execution time out of bound")
	require.Nil(t, txn)

	txn, err = block.NewTransactionData(
		logical.EndOfBlockExecutionTime,
		state.DefaultParameters())
	require.ErrorContains(t, err, "execution time out of bound")
	require.Nil(t, txn)

	txn, err = block.NewTransactionData(
		snapshotTime-1,
		state.DefaultParameters())
	require.ErrorContains(t, err, "snapshot > execution: 5 > 4")
	require.Nil(t, txn)
}

func testBlockDataValidate(
	t *testing.T,
	shouldFinalize bool,
) {
	baseSnapshotTime := logical.Time(11)
	block := NewBlockData(nil, baseSnapshotTime)

	// Commit a key before the actual test txn (which read the same key).

	testSetupTxn, err := block.NewTransactionData(
		baseSnapshotTime,
		state.DefaultParameters())
	require.NoError(t, err)

	registerId1 := flow.RegisterID{
		Owner: "",
		Key:   "key1",
	}
	expectedValue1 := flow.RegisterValue([]byte("value1"))

	err = testSetupTxn.Set(registerId1, expectedValue1)
	require.NoError(t, err)

	err = testSetupTxn.Finalize()
	require.NoError(t, err)

	_, err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(
		t,
		baseSnapshotTime+1,
		block.LatestSnapshot().SnapshotTime())

	value, err := block.LatestSnapshot().Get(registerId1)
	require.NoError(t, err)
	require.Equal(t, expectedValue1, value)

	// Start the test transaction at an "older" snapshot to ensure valdiate
	// works as expected.

	testTxn, err := block.NewTransactionData(
		baseSnapshotTime+3,
		state.DefaultParameters())
	require.NoError(t, err)

	// Commit a bunch of unrelated transactions.

	testSetupTxn, err = block.NewTransactionData(
		baseSnapshotTime+1,
		state.DefaultParameters())
	require.NoError(t, err)

	registerId2 := flow.RegisterID{
		Owner: "",
		Key:   "key2",
	}
	expectedValue2 := flow.RegisterValue([]byte("value2"))

	err = testSetupTxn.Set(registerId2, expectedValue2)
	require.NoError(t, err)

	err = testSetupTxn.Finalize()
	require.NoError(t, err)

	_, err = testSetupTxn.Commit()
	require.NoError(t, err)

	testSetupTxn, err = block.NewTransactionData(
		baseSnapshotTime+2,
		state.DefaultParameters())
	require.NoError(t, err)

	registerId3 := flow.RegisterID{
		Owner: "",
		Key:   "key3",
	}
	expectedValue3 := flow.RegisterValue([]byte("value3"))

	err = testSetupTxn.Set(registerId3, expectedValue3)
	require.NoError(t, err)

	err = testSetupTxn.Finalize()
	require.NoError(t, err)

	_, err = testSetupTxn.Commit()
	require.NoError(t, err)

	// Actual test

	_, err = testTxn.Get(registerId1)
	require.NoError(t, err)

	if shouldFinalize {
		err = testTxn.Finalize()
		require.NoError(t, err)

		require.NotNil(t, testTxn.finalizedExecutionSnapshot)
	} else {
		require.Nil(t, testTxn.finalizedExecutionSnapshot)
	}

	// Check the original snapshot tree before calling validate.
	require.Equal(t, baseSnapshotTime+1, testTxn.SnapshotTime())

	value, err = testTxn.snapshot.Get(registerId1)
	require.NoError(t, err)
	require.Equal(t, expectedValue1, value)

	value, err = testTxn.snapshot.Get(registerId2)
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = testTxn.snapshot.Get(registerId3)
	require.NoError(t, err)
	require.Nil(t, value)

	// Validate should not detect any conflict and should rebase the snapshot.
	err = testTxn.Validate()
	require.NoError(t, err)

	// Ensure validate rebase to a new snapshot tree.
	require.Equal(t, baseSnapshotTime+3, testTxn.SnapshotTime())

	value, err = testTxn.snapshot.Get(registerId1)
	require.NoError(t, err)
	require.Equal(t, expectedValue1, value)

	value, err = testTxn.snapshot.Get(registerId2)
	require.NoError(t, err)
	require.Equal(t, expectedValue2, value)

	value, err = testTxn.snapshot.Get(registerId3)
	require.NoError(t, err)
	require.Equal(t, expectedValue3, value)

	// Note: we can't make additional Get calls on a finalized transaction.
	if shouldFinalize {
		_, err = testTxn.Get(registerId1)
		require.ErrorContains(t, err, "cannot Get on a finalized state")

		_, err = testTxn.Get(registerId2)
		require.ErrorContains(t, err, "cannot Get on a finalized state")

		_, err = testTxn.Get(registerId3)
		require.ErrorContains(t, err, "cannot Get on a finalized state")
	} else {
		value, err = testTxn.Get(registerId1)
		require.NoError(t, err)
		require.Equal(t, expectedValue1, value)

		value, err = testTxn.Get(registerId2)
		require.NoError(t, err)
		require.Equal(t, expectedValue2, value)

		value, err = testTxn.Get(registerId3)
		require.NoError(t, err)
		require.Equal(t, expectedValue3, value)
	}
}

func TestBlockDataValidateInterim(t *testing.T) {
	testBlockDataValidate(t, false)
}

func TestBlockDataValidateFinalized(t *testing.T) {
	testBlockDataValidate(t, true)
}

func testBlockDataValidateRejectConflict(
	t *testing.T,
	shouldFinalize bool,
	conflictTxn int, // [1, 2, 3]
) {
	baseSnapshotTime := logical.Time(32)
	block := NewBlockData(nil, baseSnapshotTime)

	// Commit a bunch of unrelated updates

	for ; baseSnapshotTime < 42; baseSnapshotTime++ {
		testSetupTxn, err := block.NewTransactionData(
			baseSnapshotTime,
			state.DefaultParameters())
		require.NoError(t, err)

		err = testSetupTxn.Set(
			flow.RegisterID{
				Owner: "",
				Key:   fmt.Sprintf("other key - %d", baseSnapshotTime),
			},
			[]byte("blah"))
		require.NoError(t, err)

		err = testSetupTxn.Finalize()
		require.NoError(t, err)

		_, err = testSetupTxn.Commit()
		require.NoError(t, err)
	}

	// Start the test transaction at an "older" snapshot to ensure valdiate
	// works as expected.

	testTxnTime := baseSnapshotTime + 3
	testTxn, err := block.NewTransactionData(
		testTxnTime,
		state.DefaultParameters())
	require.NoError(t, err)

	// Commit one key per test setup transaction.  One of these keys will
	// conflicts with the test txn.

	txn1Time := baseSnapshotTime
	testSetupTxn, err := block.NewTransactionData(
		txn1Time,
		state.DefaultParameters())
	require.NoError(t, err)

	registerId1 := flow.RegisterID{
		Owner: "",
		Key:   "key1",
	}

	err = testSetupTxn.Set(registerId1, []byte("value1"))
	require.NoError(t, err)

	err = testSetupTxn.Finalize()
	require.NoError(t, err)

	_, err = testSetupTxn.Commit()
	require.NoError(t, err)

	txn2Time := baseSnapshotTime + 1
	testSetupTxn, err = block.NewTransactionData(
		txn2Time,
		state.DefaultParameters())
	require.NoError(t, err)

	registerId2 := flow.RegisterID{
		Owner: "",
		Key:   "key2",
	}

	err = testSetupTxn.Set(registerId2, []byte("value2"))
	require.NoError(t, err)

	err = testSetupTxn.Finalize()
	require.NoError(t, err)

	_, err = testSetupTxn.Commit()
	require.NoError(t, err)

	txn3Time := baseSnapshotTime + 2
	testSetupTxn, err = block.NewTransactionData(
		txn3Time,
		state.DefaultParameters())
	require.NoError(t, err)

	registerId3 := flow.RegisterID{
		Owner: "",
		Key:   "key3",
	}

	err = testSetupTxn.Set(registerId3, []byte("value3"))
	require.NoError(t, err)

	err = testSetupTxn.Finalize()
	require.NoError(t, err)

	_, err = testSetupTxn.Commit()
	require.NoError(t, err)

	// Actual test

	var conflictTxnTime logical.Time
	var conflictRegisterId flow.RegisterID
	switch conflictTxn {
	case 1:
		conflictTxnTime = txn1Time
		conflictRegisterId = registerId1
	case 2:
		conflictTxnTime = txn2Time
		conflictRegisterId = registerId2
	case 3:
		conflictTxnTime = txn3Time
		conflictRegisterId = registerId3
	}

	value, err := testTxn.Get(conflictRegisterId)
	require.NoError(t, err)
	require.Nil(t, value)

	if shouldFinalize {
		err = testTxn.Finalize()
		require.NoError(t, err)

		require.NotNil(t, testTxn.finalizedExecutionSnapshot)
	} else {
		require.Nil(t, testTxn.finalizedExecutionSnapshot)
	}

	// Check the original snapshot tree before calling validate.
	require.Equal(t, baseSnapshotTime, testTxn.SnapshotTime())

	err = testTxn.Validate()
	require.ErrorContains(
		t,
		err,
		fmt.Sprintf(
			conflictErrorTemplate,
			conflictTxnTime,
			testTxnTime,
			baseSnapshotTime,
			conflictRegisterId))
	require.True(t, errors.IsRetryableConflictError(err))

	// Validate should not rebase the snapshot tree on error
	require.Equal(t, baseSnapshotTime, testTxn.SnapshotTime())
}

func TestBlockDataValidateInterimRejectConflict(t *testing.T) {
	testBlockDataValidateRejectConflict(t, false, 1)
	testBlockDataValidateRejectConflict(t, false, 2)
	testBlockDataValidateRejectConflict(t, false, 3)
}

func TestBlockDataValidateFinalizedRejectConflict(t *testing.T) {
	testBlockDataValidateRejectConflict(t, true, 1)
	testBlockDataValidateRejectConflict(t, true, 2)
	testBlockDataValidateRejectConflict(t, true, 3)
}

func TestBlockDataCommit(t *testing.T) {
	block := NewBlockData(nil, 0)

	// Start test txn at an "older" snapshot.
	txn, err := block.NewTransactionData(3, state.DefaultParameters())
	require.NoError(t, err)

	// Commit a bunch of unrelated updates

	for i := logical.Time(0); i < 3; i++ {
		testSetupTxn, err := block.NewTransactionData(
			i,
			state.DefaultParameters())
		require.NoError(t, err)

		err = testSetupTxn.Set(
			flow.RegisterID{
				Owner: "",
				Key:   fmt.Sprintf("other key - %d", i),
			},
			[]byte("blah"))
		require.NoError(t, err)

		err = testSetupTxn.Finalize()
		require.NoError(t, err)

		_, err = testSetupTxn.Commit()
		require.NoError(t, err)
	}

	// "resume" test txn

	writeRegisterId := flow.RegisterID{
		Owner: "",
		Key:   "write",
	}
	expectedValue := flow.RegisterValue([]byte("value"))

	err = txn.Set(writeRegisterId, expectedValue)
	require.NoError(t, err)

	readRegisterId := flow.RegisterID{
		Owner: "",
		Key:   "read",
	}
	value, err := txn.Get(readRegisterId)
	require.NoError(t, err)
	require.Nil(t, value)

	err = txn.Finalize()
	require.NoError(t, err)

	// Actual test.  Ensure the transaction is committed.

	require.Equal(t, logical.Time(0), txn.SnapshotTime())
	require.Equal(t, logical.Time(3), block.LatestSnapshot().SnapshotTime())

	executionSnapshot, err := txn.Commit()
	require.NoError(t, err)
	require.NotNil(t, executionSnapshot)
	require.Equal(
		t,
		map[flow.RegisterID]struct{}{
			readRegisterId: struct{}{},
		},
		executionSnapshot.ReadSet)
	require.Equal(
		t,
		map[flow.RegisterID]flow.RegisterValue{
			writeRegisterId: expectedValue,
		},
		executionSnapshot.WriteSet)

	require.Equal(t, logical.Time(4), block.LatestSnapshot().SnapshotTime())

	value, err = block.LatestSnapshot().Get(writeRegisterId)
	require.NoError(t, err)
	require.Equal(t, expectedValue, value)
}

func TestBlockDataCommitSnapshotReadDontAdvanceTime(t *testing.T) {
	baseRegisterId := flow.RegisterID{
		Owner: "",
		Key:   "base",
	}
	baseValue := flow.RegisterValue([]byte("original"))

	baseSnapshotTime := logical.Time(16)

	block := NewBlockData(
		snapshot.MapStorageSnapshot{
			baseRegisterId: baseValue,
		},
		baseSnapshotTime)

	txn := block.NewSnapshotReadTransactionData(state.DefaultParameters())

	readRegisterId := flow.RegisterID{
		Owner: "",
		Key:   "read",
	}
	value, err := txn.Get(readRegisterId)
	require.NoError(t, err)
	require.Nil(t, value)

	err = txn.Set(baseRegisterId, []byte("bad"))
	require.NoError(t, err)

	err = txn.Finalize()
	require.NoError(t, err)

	require.Equal(t, baseSnapshotTime, block.LatestSnapshot().SnapshotTime())

	executionSnapshot, err := txn.Commit()
	require.NoError(t, err)

	require.NotNil(t, executionSnapshot)

	require.Equal(
		t,
		map[flow.RegisterID]struct{}{
			readRegisterId: struct{}{},
		},
		executionSnapshot.ReadSet)

	// Ensure we have dropped the write set internally.
	require.Nil(t, executionSnapshot.WriteSet)

	// Ensure block snapshot is not updated.
	require.Equal(t, baseSnapshotTime, block.LatestSnapshot().SnapshotTime())

	value, err = block.LatestSnapshot().Get(baseRegisterId)
	require.NoError(t, err)
	require.Equal(t, baseValue, value)
}

func TestBlockDataCommitRejectNotFinalized(t *testing.T) {
	block := NewBlockData(nil, 0)

	txn, err := block.NewTransactionData(0, state.DefaultParameters())
	require.NoError(t, err)

	executionSnapshot, err := txn.Commit()
	require.ErrorContains(t, err, "transaction not finalized")
	require.False(t, errors.IsRetryableConflictError(err))
	require.Nil(t, executionSnapshot)
}

func TestBlockDataCommitRejectConflict(t *testing.T) {
	block := NewBlockData(nil, 0)

	registerId := flow.RegisterID{
		Owner: "",
		Key:   "key1",
	}

	// Start test txn at an "older" snapshot.
	testTxn, err := block.NewTransactionData(1, state.DefaultParameters())
	require.NoError(t, err)

	// Commit a conflicting key
	testSetupTxn, err := block.NewTransactionData(0, state.DefaultParameters())
	require.NoError(t, err)

	err = testSetupTxn.Set(registerId, []byte("value"))
	require.NoError(t, err)

	err = testSetupTxn.Finalize()
	require.NoError(t, err)

	executionSnapshot, err := testSetupTxn.Commit()
	require.NoError(t, err)
	require.NotNil(t, executionSnapshot)

	// Actual test

	require.Equal(t, logical.Time(1), block.LatestSnapshot().SnapshotTime())

	value, err := testTxn.Get(registerId)
	require.NoError(t, err)
	require.Nil(t, value)

	err = testTxn.Finalize()
	require.NoError(t, err)

	executionSnapshot, err = testTxn.Commit()
	require.Error(t, err)
	require.True(t, errors.IsRetryableConflictError(err))
	require.Nil(t, executionSnapshot)

	// testTxn is not committed to block.
	require.Equal(t, logical.Time(1), block.LatestSnapshot().SnapshotTime())
}

func TestBlockDataCommitRejectCommitGap(t *testing.T) {
	block := NewBlockData(nil, 1)

	for i := logical.Time(2); i < 5; i++ {
		txn, err := block.NewTransactionData(i, state.DefaultParameters())
		require.NoError(t, err)

		err = txn.Finalize()
		require.NoError(t, err)

		executionSnapshot, err := txn.Commit()
		require.ErrorContains(
			t,
			err,
			fmt.Sprintf("missing commit range [1, %d)", i))
		require.False(t, errors.IsRetryableConflictError(err))
		require.Nil(t, executionSnapshot)

		// testTxn is not committed to block.
		require.Equal(t, logical.Time(1), block.LatestSnapshot().SnapshotTime())
	}
}

func TestBlockDataCommitRejectNonIncreasingExecutionTime1(t *testing.T) {
	block := NewBlockData(nil, 0)

	testTxn, err := block.NewTransactionData(5, state.DefaultParameters())
	require.NoError(t, err)

	err = testTxn.Finalize()
	require.NoError(t, err)

	// Commit a bunch of unrelated transactions.
	for i := logical.Time(0); i < 10; i++ {
		txn, err := block.NewTransactionData(i, state.DefaultParameters())
		require.NoError(t, err)

		err = txn.Finalize()
		require.NoError(t, err)

		_, err = txn.Commit()
		require.NoError(t, err)
	}

	// sanity check before testing commit.
	require.Equal(t, logical.Time(10), block.LatestSnapshot().SnapshotTime())

	// "re-commit" an already committed transaction
	executionSnapshot, err := testTxn.Commit()
	require.ErrorContains(t, err, "non-increasing time (9 >= 5)")
	require.False(t, errors.IsRetryableConflictError(err))
	require.Nil(t, executionSnapshot)

	// testTxn is not committed to block.
	require.Equal(t, logical.Time(10), block.LatestSnapshot().SnapshotTime())
}

func TestBlockDataCommitRejectNonIncreasingExecutionTime2(t *testing.T) {
	block := NewBlockData(nil, 13)

	testTxn, err := block.NewTransactionData(13, state.DefaultParameters())
	require.NoError(t, err)

	err = testTxn.Finalize()
	require.NoError(t, err)

	executionSnapshot, err := testTxn.Commit()
	require.NoError(t, err)
	require.NotNil(t, executionSnapshot)

	// "re-commit" an already committed transaction
	executionSnapshot, err = testTxn.Commit()
	require.ErrorContains(t, err, "non-increasing time (13 >= 13)")
	require.False(t, errors.IsRetryableConflictError(err))
	require.Nil(t, executionSnapshot)
}

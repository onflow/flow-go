package derived

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func newEmptyTestBlock() *DerivedDataTable[string, *string] {
	return NewEmptyTable[string, *string]()
}

func TestDerivedDataTableWithTransactionOffset(t *testing.T) {
	block := NewEmptyTableWithOffset[string, *string](18)

	require.Equal(
		t,
		LogicalTime(17),
		block.LatestCommitExecutionTimeForTestingOnly())
}

func TestDerivedDataTableNormalTransactionInvalidExecutionTimeBound(
	t *testing.T,
) {
	block := newEmptyTestBlock()

	_, err := block.NewTableTransaction(-1, -1)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	_, err = block.NewTableTransaction(0, EndOfBlockExecutionTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewTableTransaction(0, EndOfBlockExecutionTime-1)
	require.NoError(t, err)
}

func TestDerivedDataTableNormalTransactionInvalidSnapshotTime(t *testing.T) {
	block := newEmptyTestBlock()

	_, err := block.NewTableTransaction(10, 0)
	require.ErrorContains(t, err, "snapshot > execution")

	_, err = block.NewTableTransaction(10, 10)
	require.NoError(t, err)

	_, err = block.NewTableTransaction(999, 998)
	require.ErrorContains(t, err, "snapshot > execution")

	_, err = block.NewTableTransaction(999, 999)
	require.NoError(t, err)
}

func TestDerivedDataTableSnapshotReadTransactionInvalidExecutionTimeBound(
	t *testing.T,
) {
	block := newEmptyTestBlock()

	_, err := block.NewSnapshotReadTableTransaction(
		ParentBlockTime,
		ParentBlockTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewSnapshotReadTableTransaction(ParentBlockTime, 0)
	require.NoError(t, err)

	_, err = block.NewSnapshotReadTableTransaction(0, ChildBlockTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewSnapshotReadTableTransaction(
		0,
		EndOfBlockExecutionTime)
	require.NoError(t, err)
}

func TestDerivedDataTableToValidateTime(t *testing.T) {
	block := NewEmptyTableWithOffset[string, *string](8)
	require.Equal(
		t,
		LogicalTime(7),
		block.LatestCommitExecutionTimeForTestingOnly())

	testTxnSnapshotTime := LogicalTime(5)

	testTxn, err := block.NewTableTransaction(testTxnSnapshotTime, 20)
	require.NoError(t, err)
	require.Equal(
		t,
		testTxnSnapshotTime,
		testTxn.ToValidateTimeForTestingOnly())

	testTxn.Set("key1", nil, nil)
	require.Equal(
		t,
		testTxnSnapshotTime,
		testTxn.ToValidateTimeForTestingOnly())

	err = testTxn.Validate()
	require.NoError(t, err)
	require.Equal(
		t,
		LogicalTime(8),
		testTxn.ToValidateTimeForTestingOnly())

	testSetupTxn, err := block.NewTableTransaction(8, 8)
	require.NoError(t, err)

	invalidator1 := &testInvalidator{invalidateName: "blah"}

	testSetupTxn.AddInvalidator(invalidator1)
	err = testSetupTxn.Commit()
	require.NoError(t, err)

	err = testTxn.Validate()
	require.NoError(t, err)
	require.Equal(
		t,
		LogicalTime(9),
		testTxn.ToValidateTimeForTestingOnly())

	require.Equal(t, 1, invalidator1.callCount)

	// Multiple transactions committed between validate calls

	testSetupTxn, err = block.NewTableTransaction(6, 9)
	require.NoError(t, err)

	invalidator2 := &testInvalidator{invalidateName: "blah"}

	testSetupTxn.AddInvalidator(invalidator2)
	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testSetupTxn, err = block.NewTableTransaction(8, 10)
	require.NoError(t, err)

	invalidator3 := &testInvalidator{invalidateName: "blah"}

	testSetupTxn.AddInvalidator(invalidator3)
	err = testSetupTxn.Commit()
	require.NoError(t, err)

	err = testTxn.Validate()
	require.NoError(t, err)
	require.Equal(
		t,
		LogicalTime(11),
		testTxn.ToValidateTimeForTestingOnly())

	require.Equal(t, 1, invalidator1.callCount)
	require.Equal(t, 1, invalidator2.callCount)
	require.Equal(t, 1, invalidator3.callCount)

	// No validate time advancement

	err = testTxn.Validate()
	require.NoError(t, err)
	require.Equal(
		t,
		LogicalTime(11),
		testTxn.ToValidateTimeForTestingOnly())

	require.Equal(t, 1, invalidator1.callCount)
	require.Equal(t, 1, invalidator2.callCount)
	require.Equal(t, 1, invalidator3.callCount)

	// Setting a value derived from snapshot time will reset the validate time

	testTxn.Set("key2", nil, nil)
	require.Equal(
		t,
		testTxnSnapshotTime,
		testTxn.ToValidateTimeForTestingOnly())

	err = testTxn.Validate()
	require.NoError(t, err)
	require.Equal(
		t,
		LogicalTime(11),
		testTxn.ToValidateTimeForTestingOnly())

	// callCount = 3 because key1 is validated twice, key2 validated once.
	require.Equal(t, 3, invalidator1.callCount)
	require.Equal(t, 3, invalidator2.callCount)
	require.Equal(t, 3, invalidator3.callCount)

	// validate error does not advance validated time

	testSetupTxn, err = block.NewTableTransaction(11, 11)
	require.NoError(t, err)

	invalidator4 := &testInvalidator{invalidateName: "blah"}

	testSetupTxn.AddInvalidator(invalidator4)
	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testSetupTxn, err = block.NewTableTransaction(12, 12)
	require.NoError(t, err)

	invalidator5 := &testInvalidator{invalidateAll: true}

	testSetupTxn.AddInvalidator(invalidator5)
	err = testSetupTxn.Commit()
	require.NoError(t, err)

	for i := 1; i < 10; i++ {
		err = testTxn.Validate()
		require.Error(t, err)
		require.Equal(
			t,
			LogicalTime(11),
			testTxn.ToValidateTimeForTestingOnly())

		require.Equal(t, 3, invalidator1.callCount)
		require.Equal(t, 3, invalidator2.callCount)
		require.Equal(t, 3, invalidator3.callCount)
		require.Equal(t, i, invalidator4.callCount)
		require.Equal(t, i, invalidator5.callCount)
	}
}

func TestDerivedDataTableOutOfOrderValidate(t *testing.T) {
	block := newEmptyTestBlock()

	testTxn1, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	testTxn2, err := block.NewTableTransaction(1, 1)
	require.NoError(t, err)

	testTxn3, err := block.NewTableTransaction(2, 2)
	require.NoError(t, err)

	testTxn4, err := block.NewTableTransaction(3, 3)
	require.NoError(t, err)

	// Validate can be called in any order as long as the transactions
	// are committed in the correct order.

	validateErr := testTxn4.Validate()
	require.NoError(t, validateErr)

	validateErr = testTxn2.Validate()
	require.NoError(t, validateErr)

	validateErr = testTxn3.Validate()
	require.NoError(t, validateErr)

	validateErr = testTxn1.Validate()
	require.NoError(t, validateErr)

	err = testTxn1.Commit()
	require.NoError(t, err)

	validateErr = testTxn2.Validate()
	require.NoError(t, validateErr)

	validateErr = testTxn3.Validate()
	require.NoError(t, validateErr)

	validateErr = testTxn4.Validate()
	require.NoError(t, validateErr)

	validateErr = testTxn2.Validate()
	require.NoError(t, validateErr)
}

func TestDerivedDataTableValidateRejectOutOfOrderCommit(t *testing.T) {
	block := newEmptyTestBlock()

	testTxn, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	testSetupTxn, err := block.NewTableTransaction(0, 1)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.NoError(t, validateErr)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	validateErr = testTxn.Validate()
	require.ErrorContains(t, validateErr, "non-increasing time")
	require.False(t, validateErr.IsRetryable())
}

func TestDerivedDataTableValidateRejectNonIncreasingExecutionTime(t *testing.T) {
	block := newEmptyTestBlock()

	testSetupTxn, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "non-increasing time")
	require.False(t, validateErr.IsRetryable())
}

func TestDerivedDataTableValidateRejectOutdatedReadSet(t *testing.T) {
	block := newEmptyTestBlock()

	testSetupTxn1, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	testSetupTxn2, err := block.NewTableTransaction(0, 1)
	require.NoError(t, err)

	testTxn, err := block.NewTableTransaction(0, 2)
	require.NoError(t, err)

	key := "abc"
	valueString := "value"
	expectedValue := &valueString
	expectedSnapshot := &state.ExecutionSnapshot{}

	testSetupTxn1.Set(key, expectedValue, expectedSnapshot)

	testSetupTxn1.AddInvalidator(&testInvalidator{})

	err = testSetupTxn1.Commit()
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.NoError(t, validateErr)

	actualProg, actualSnapshot, ok := testTxn.Get(key)
	require.True(t, ok)
	require.Same(t, expectedValue, actualProg)
	require.Same(t, expectedSnapshot, actualSnapshot)

	validateErr = testTxn.Validate()
	require.NoError(t, validateErr)

	testSetupTxn2.AddInvalidator(&testInvalidator{invalidateAll: true})

	err = testSetupTxn2.Commit()
	require.NoError(t, err)

	validateErr = testTxn.Validate()
	require.ErrorContains(t, validateErr, "outdated read set")
	require.True(t, validateErr.IsRetryable())
}

func TestDerivedDataTableValidateRejectOutdatedWriteSet(t *testing.T) {
	block := newEmptyTestBlock()

	testSetupTxn, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	testSetupTxn.AddInvalidator(&testInvalidator{invalidateAll: true})

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, 1, len(block.InvalidatorsForTestingOnly()))

	testTxn, err := block.NewTableTransaction(0, 1)
	require.NoError(t, err)

	value := "value"
	testTxn.Set("key", &value, &state.ExecutionSnapshot{})

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "outdated write set")
	require.True(t, validateErr.IsRetryable())
}

func TestDerivedDataTableValidateIgnoreInvalidatorsOlderThanSnapshot(t *testing.T) {
	block := newEmptyTestBlock()

	testSetupTxn, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	testSetupTxn.AddInvalidator(&testInvalidator{invalidateAll: true})
	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, 1, len(block.InvalidatorsForTestingOnly()))

	testTxn, err := block.NewTableTransaction(1, 1)
	require.NoError(t, err)

	value := "value"
	testTxn.Set("key", &value, &state.ExecutionSnapshot{})

	err = testTxn.Validate()
	require.NoError(t, err)
}

func TestDerivedDataTableCommitEndOfBlockSnapshotRead(t *testing.T) {
	block := newEmptyTestBlock()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewTableTransaction(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewSnapshotReadTableTransaction(
		EndOfBlockExecutionTime,
		EndOfBlockExecutionTime)
	require.NoError(t, err)

	err = testTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())
}

func TestDerivedDataTableCommitSnapshotReadDontAdvanceTime(t *testing.T) {
	block := newEmptyTestBlock()

	commitTime := LogicalTime(71)
	testSetupTxn, err := block.NewTableTransaction(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	repeatedTime := commitTime + 1
	for i := 0; i < 10; i++ {
		txn, err := block.NewSnapshotReadTableTransaction(0, repeatedTime)
		require.NoError(t, err)

		err = txn.Commit()
		require.NoError(t, err)
	}

	require.Equal(
		t,
		commitTime,
		block.LatestCommitExecutionTimeForTestingOnly())
}

func TestDerivedDataTableCommitWriteOnlyTransactionNoInvalidation(t *testing.T) {
	block := newEmptyTestBlock()

	testTxn, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	key := "234"

	actualValue, actualSnapshot, ok := testTxn.Get(key)
	require.False(t, ok)
	require.Nil(t, actualValue)
	require.Nil(t, actualSnapshot)

	valueString := "stuff"
	expectedValue := &valueString
	expectedSnapshot := &state.ExecutionSnapshot{}

	testTxn.Set(key, expectedValue, expectedSnapshot)

	actualValue, actualSnapshot, ok = testTxn.Get(key)
	require.True(t, ok)
	require.Same(t, expectedValue, actualValue)
	require.Same(t, expectedSnapshot, actualSnapshot)

	testTxn.AddInvalidator(&testInvalidator{})

	err = testTxn.Commit()
	require.NoError(t, err)

	// Sanity check

	require.Equal(
		t,
		LogicalTime(0),
		block.LatestCommitExecutionTimeForTestingOnly())

	require.Equal(t, 0, len(block.InvalidatorsForTestingOnly()))

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 1, len(entries))

	entry, ok := entries[key]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, expectedValue, entry.Value)
	require.Same(t, expectedSnapshot, entry.ExecutionSnapshot)
}

func TestDerivedDataTableCommitWriteOnlyTransactionWithInvalidation(t *testing.T) {
	block := newEmptyTestBlock()

	testTxnTime := LogicalTime(47)
	testTxn, err := block.NewTableTransaction(0, testTxnTime)
	require.NoError(t, err)

	key := "999"

	actualValue, actualSnapshot, ok := testTxn.Get(key)
	require.False(t, ok)
	require.Nil(t, actualValue)
	require.Nil(t, actualSnapshot)

	valueString := "blah"
	expectedValue := &valueString
	expectedSnapshot := &state.ExecutionSnapshot{}

	testTxn.Set(key, expectedValue, expectedSnapshot)

	actualValue, actualSnapshot, ok = testTxn.Get(key)
	require.True(t, ok)
	require.Same(t, expectedValue, actualValue)
	require.Same(t, expectedSnapshot, actualSnapshot)

	invalidator := &testInvalidator{invalidateAll: true}

	testTxn.AddInvalidator(invalidator)

	err = testTxn.Commit()
	require.NoError(t, err)

	// Sanity check

	require.Equal(
		t,
		testTxnTime,
		block.LatestCommitExecutionTimeForTestingOnly())

	require.Equal(
		t,
		chainedTableInvalidators[string, *string]{
			{
				TableInvalidator: invalidator,
				executionTime:    testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	require.Equal(t, 0, len(block.EntriesForTestingOnly()))
}

func TestDerivedDataTableCommitUseOriginalEntryOnDuplicateWriteEntries(t *testing.T) {
	block := newEmptyTestBlock()

	testSetupTxn, err := block.NewTableTransaction(0, 11)
	require.NoError(t, err)

	testTxn, err := block.NewTableTransaction(10, 12)
	require.NoError(t, err)

	key := "17"
	valueString := "foo"
	expectedValue := &valueString
	expectedSnapshot := &state.ExecutionSnapshot{}

	testSetupTxn.Set(key, expectedValue, expectedSnapshot)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 1, len(entries))

	expectedEntry, ok := entries[key]
	require.True(t, ok)

	otherString := "other"
	otherValue := &otherString
	otherSnapshot := &state.ExecutionSnapshot{}

	testTxn.Set(key, otherValue, otherSnapshot)

	err = testTxn.Commit()
	require.NoError(t, err)

	entries = block.EntriesForTestingOnly()
	require.Equal(t, 1, len(entries))

	actualEntry, ok := entries[key]
	require.True(t, ok)

	require.Same(t, expectedEntry, actualEntry)
	require.False(t, actualEntry.isInvalid)
	require.Same(t, expectedValue, actualEntry.Value)
	require.Same(t, expectedSnapshot, actualEntry.ExecutionSnapshot)
	require.NotSame(t, otherValue, actualEntry.Value)
	require.NotSame(t, otherSnapshot, actualEntry.ExecutionSnapshot)
}

func TestDerivedDataTableCommitReadOnlyTransactionNoInvalidation(t *testing.T) {
	block := newEmptyTestBlock()

	testSetupTxn, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	testTxn, err := block.NewTableTransaction(0, 1)
	require.NoError(t, err)

	key1 := "key1"
	valStr1 := "value1"
	expectedValue1 := &valStr1
	expectedSnapshot1 := &state.ExecutionSnapshot{}

	testSetupTxn.Set(key1, expectedValue1, expectedSnapshot1)

	key2 := "key2"
	valStr2 := "value2"
	expectedValue2 := &valStr2
	expectedSnapshot2 := &state.ExecutionSnapshot{}

	testSetupTxn.Set(key2, expectedValue2, expectedSnapshot2)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	actualValue, actualSnapshot, ok := testTxn.Get(key1)
	require.True(t, ok)
	require.Same(t, expectedValue1, actualValue)
	require.Same(t, expectedSnapshot1, actualSnapshot)

	actualValue, actualSnapshot, ok = testTxn.Get(key2)
	require.True(t, ok)
	require.Same(t, expectedValue2, actualValue)
	require.Same(t, expectedSnapshot2, actualSnapshot)

	actualValue, actualSnapshot, ok = testTxn.Get("key3")
	require.False(t, ok)
	require.Nil(t, actualValue)
	require.Nil(t, actualSnapshot)

	testTxn.AddInvalidator(&testInvalidator{})

	err = testTxn.Commit()
	require.NoError(t, err)

	// Sanity check

	require.Equal(
		t,
		LogicalTime(1),
		block.LatestCommitExecutionTimeForTestingOnly())

	require.Equal(t, 0, len(block.InvalidatorsForTestingOnly()))

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 2, len(entries))

	entry, ok := entries[key1]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, expectedValue1, entry.Value)
	require.Same(t, expectedSnapshot1, entry.ExecutionSnapshot)

	entry, ok = entries[key2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, expectedValue2, entry.Value)
	require.Same(t, expectedSnapshot2, entry.ExecutionSnapshot)
}

func TestDerivedDataTableCommitReadOnlyTransactionWithInvalidation(t *testing.T) {
	block := newEmptyTestBlock()

	testSetupTxn1Time := LogicalTime(2)
	testSetupTxn1, err := block.NewTableTransaction(0, testSetupTxn1Time)
	require.NoError(t, err)

	testSetupTxn2, err := block.NewTableTransaction(0, 4)
	require.NoError(t, err)

	testTxnTime := LogicalTime(6)
	testTxn, err := block.NewTableTransaction(0, testTxnTime)
	require.NoError(t, err)

	testSetupTxn1Invalidator := &testInvalidator{
		invalidateName: "blah",
	}
	testSetupTxn1.AddInvalidator(testSetupTxn1Invalidator)

	err = testSetupTxn1.Commit()
	require.NoError(t, err)

	key1 := "key1"
	valStr1 := "v1"
	expectedValue1 := &valStr1
	expectedSnapshot1 := &state.ExecutionSnapshot{}

	testSetupTxn2.Set(key1, expectedValue1, expectedSnapshot1)

	key2 := "key2"
	valStr2 := "v2"
	expectedValue2 := &valStr2
	expectedSnapshot2 := &state.ExecutionSnapshot{}

	testSetupTxn2.Set(key2, expectedValue2, expectedSnapshot2)

	err = testSetupTxn2.Commit()
	require.NoError(t, err)

	actualValue, actualSnapshot, ok := testTxn.Get(key1)
	require.True(t, ok)
	require.Same(t, expectedValue1, actualValue)
	require.Same(t, expectedSnapshot1, actualSnapshot)

	actualValue, actualSnapshot, ok = testTxn.Get(key2)
	require.True(t, ok)
	require.Same(t, expectedValue2, actualValue)
	require.Same(t, expectedSnapshot2, actualSnapshot)

	actualValue, actualSnapshot, ok = testTxn.Get("key3")
	require.False(t, ok)
	require.Nil(t, actualValue)
	require.Nil(t, actualSnapshot)

	testTxnInvalidator := &testInvalidator{invalidateAll: true}
	testTxn.AddInvalidator(testTxnInvalidator)

	err = testTxn.Commit()
	require.NoError(t, err)

	// Sanity check

	require.Equal(
		t,
		testTxnTime,
		block.LatestCommitExecutionTimeForTestingOnly())

	require.Equal(
		t,
		chainedTableInvalidators[string, *string]{
			{
				TableInvalidator: testSetupTxn1Invalidator,
				executionTime:    testSetupTxn1Time,
			},
			{
				TableInvalidator: testTxnInvalidator,
				executionTime:    testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	require.Equal(t, 0, len(block.EntriesForTestingOnly()))
}

func TestDerivedDataTableCommitValidateError(t *testing.T) {
	block := newEmptyTestBlock()

	testSetupTxn, err := block.NewTableTransaction(0, 10)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewTableTransaction(10, 10)
	require.NoError(t, err)

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "non-increasing time")
	require.False(t, commitErr.IsRetryable())
}

func TestDerivedDataTableCommitRejectCommitGapForNormalTxn(t *testing.T) {
	block := newEmptyTestBlock()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewTableTransaction(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(
		t,
		commitTime,
		block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewTableTransaction(10, 10)
	require.NoError(t, err)

	err = testTxn.Validate()
	require.NoError(t, err)

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "missing commit range [6, 10)")
	require.False(t, commitErr.IsRetryable())
}

func TestDerivedDataTableCommitRejectCommitGapForSnapshotRead(t *testing.T) {
	block := newEmptyTestBlock()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewTableTransaction(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(
		t,
		commitTime,
		block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewSnapshotReadTableTransaction(10, 10)
	require.NoError(t, err)

	err = testTxn.Validate()
	require.NoError(t, err)

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "missing commit range [6, 10)")
	require.False(t, commitErr.IsRetryable())
}

func TestDerivedDataTableCommitSnapshotReadDoesNotAdvanceCommitTime(t *testing.T) {
	block := newEmptyTestBlock()

	expectedTime := LogicalTime(10)
	testSetupTxn, err := block.NewTableTransaction(0, expectedTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewSnapshotReadTableTransaction(0, 11)
	require.NoError(t, err)

	err = testTxn.Commit()
	require.NoError(t, err)

	require.Equal(
		t,
		expectedTime,
		block.LatestCommitExecutionTimeForTestingOnly())
}

func TestDerivedDataTableCommitBadSnapshotReadInvalidator(t *testing.T) {
	block := newEmptyTestBlock()

	testTxn, err := block.NewSnapshotReadTableTransaction(0, 42)
	require.NoError(t, err)

	testTxn.AddInvalidator(&testInvalidator{invalidateAll: true})

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "snapshot read can't invalidate")
	require.False(t, commitErr.IsRetryable())
}

func TestDerivedDataTableCommitFineGrainInvalidation(t *testing.T) {
	block := newEmptyTestBlock()

	// Setup the database with two read entries

	testSetupTxn, err := block.NewTableTransaction(0, 0)
	require.NoError(t, err)

	readKey1 := "read-key-1"
	readValStr1 := "read-value-1"
	readValue1 := &readValStr1
	readSnapshot1 := &state.ExecutionSnapshot{}

	readKey2 := "read-key-2"
	readValStr2 := "read-value-2"
	readValue2 := &readValStr2
	readSnapshot2 := &state.ExecutionSnapshot{}

	testSetupTxn.Set(readKey1, readValue1, readSnapshot1)
	testSetupTxn.Set(readKey2, readValue2, readSnapshot2)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	// Setup the test transaction by read both existing entries and writing
	// two new ones,

	testTxnTime := LogicalTime(15)
	testTxn, err := block.NewTableTransaction(1, testTxnTime)
	require.NoError(t, err)

	actualValue, actualSnapshot, ok := testTxn.Get(readKey1)
	require.True(t, ok)
	require.Same(t, readValue1, actualValue)
	require.Same(t, readSnapshot1, actualSnapshot)

	actualValue, actualSnapshot, ok = testTxn.Get(readKey2)
	require.True(t, ok)
	require.Same(t, readValue2, actualValue)
	require.Same(t, readSnapshot2, actualSnapshot)

	writeKey1 := "write key 1"
	writeValStr1 := "write value 1"
	writeValue1 := &writeValStr1
	writeSnapshot1 := &state.ExecutionSnapshot{}

	writeKey2 := "write key 2"
	writeValStr2 := "write value 2"
	writeValue2 := &writeValStr2
	writeSnapshot2 := &state.ExecutionSnapshot{}

	testTxn.Set(writeKey1, writeValue1, writeSnapshot1)
	testTxn.Set(writeKey2, writeValue2, writeSnapshot2)

	// Actual test.  Invalidate one pre-existing entry and one new entry.

	invalidator1 := &testInvalidator{
		invalidateName: readKey1,
	}
	invalidator2 := &testInvalidator{
		invalidateName: writeKey1,
	}
	testTxn.AddInvalidator(nil)
	testTxn.AddInvalidator(invalidator1)
	testTxn.AddInvalidator(&testInvalidator{})
	testTxn.AddInvalidator(invalidator2)
	testTxn.AddInvalidator(&testInvalidator{})

	err = testTxn.Commit()
	require.NoError(t, err)

	require.Equal(
		t,
		testTxnTime,
		block.LatestCommitExecutionTimeForTestingOnly())

	require.Equal(
		t,
		chainedTableInvalidators[string, *string]{
			{
				TableInvalidator: invalidator1,
				executionTime:    testTxnTime,
			},
			{
				TableInvalidator: invalidator2,
				executionTime:    testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 2, len(entries))

	entry, ok := entries[readKey2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, readValue2, entry.Value)
	require.Same(t, readSnapshot2, entry.ExecutionSnapshot)

	entry, ok = entries[writeKey2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, writeValue2, entry.Value)
	require.Same(t, writeSnapshot2, entry.ExecutionSnapshot)
}

func TestDerivedDataTableNewChildDerivedBlockData(t *testing.T) {
	parentBlock := newEmptyTestBlock()

	require.Equal(
		t,
		ParentBlockTime,
		parentBlock.LatestCommitExecutionTimeForTestingOnly())
	require.Equal(t, 0, len(parentBlock.InvalidatorsForTestingOnly()))
	require.Equal(t, 0, len(parentBlock.EntriesForTestingOnly()))

	txn, err := parentBlock.NewTableTransaction(0, 0)
	require.NoError(t, err)

	txn.AddInvalidator(&testInvalidator{invalidateAll: true})

	err = txn.Commit()
	require.NoError(t, err)

	txn, err = parentBlock.NewTableTransaction(1, 1)
	require.NoError(t, err)

	key := "foo bar"
	valStr := "zzz"
	value := &valStr
	state := &state.ExecutionSnapshot{}

	txn.Set(key, value, state)

	err = txn.Commit()
	require.NoError(t, err)

	// Sanity check parent block

	require.Equal(
		t,
		LogicalTime(1),
		parentBlock.LatestCommitExecutionTimeForTestingOnly())

	require.Equal(t, 1, len(parentBlock.InvalidatorsForTestingOnly()))

	parentEntries := parentBlock.EntriesForTestingOnly()
	require.Equal(t, 1, len(parentEntries))

	parentEntry, ok := parentEntries[key]
	require.True(t, ok)
	require.False(t, parentEntry.isInvalid)
	require.Same(t, value, parentEntry.Value)
	require.Same(t, state, parentEntry.ExecutionSnapshot)

	// Verify child is correctly initialized

	childBlock := parentBlock.NewChildTable()

	require.Equal(
		t,
		ParentBlockTime,
		childBlock.LatestCommitExecutionTimeForTestingOnly())

	require.Equal(t, 0, len(childBlock.InvalidatorsForTestingOnly()))

	childEntries := childBlock.EntriesForTestingOnly()
	require.Equal(t, 1, len(childEntries))

	childEntry, ok := childEntries[key]
	require.True(t, ok)
	require.False(t, childEntry.isInvalid)
	require.Same(t, value, childEntry.Value)
	require.Same(t, state, childEntry.ExecutionSnapshot)

	require.NotSame(t, parentEntry, childEntry)
}

type testValueComputer struct {
	valueFunc func() (int, error)
	called    bool
}

func (computer *testValueComputer) Compute(
	txnState state.NestedTransaction,
	key flow.RegisterID,
) (
	int,
	error,
) {
	computer.called = true
	_, err := txnState.Get(key)
	if err != nil {
		return 0, err
	}

	return computer.valueFunc()
}

func TestDerivedDataTableGetOrCompute(t *testing.T) {
	blockDerivedData := NewEmptyTable[flow.RegisterID, int]()

	key := flow.NewRegisterID("addr", "key")
	value := 12345

	t.Run("compute value", func(t *testing.T) {
		view := delta.NewDeltaView(nil)
		txnState := state.NewTransactionState(view, state.DefaultParameters())

		txnDerivedData, err := blockDerivedData.NewTableTransaction(0, 0)
		assert.NoError(t, err)

		// first attempt to compute the value returns an error.
		// But it's perfectly safe to handle the error and try again with the same txnState.
		computer := &testValueComputer{
			valueFunc: func() (int, error) { return 0, fmt.Errorf("compute error") },
		}
		_, err = txnDerivedData.GetOrCompute(txnState, key, computer)
		assert.Error(t, err)
		assert.Equal(t, 0, txnState.NumNestedTransactions())

		// second attempt to compute the value succeeds.

		computer = &testValueComputer{
			valueFunc: func() (int, error) { return value, nil },
		}
		val, err := txnDerivedData.GetOrCompute(txnState, key, computer)
		assert.NoError(t, err)
		assert.Equal(t, value, val)
		assert.True(t, computer.called)

		_, found := view.Finalize().ReadSet[key]
		assert.True(t, found)

		// Commit to setup the next test.
		err = txnDerivedData.Commit()
		assert.Nil(t, err)
	})

	t.Run("get value", func(t *testing.T) {
		view := delta.NewDeltaView(nil)
		txnState := state.NewTransactionState(view, state.DefaultParameters())

		txnDerivedData, err := blockDerivedData.NewTableTransaction(1, 1)
		assert.NoError(t, err)

		computer := &testValueComputer{
			valueFunc: func() (int, error) { return value, nil },
		}
		val, err := txnDerivedData.GetOrCompute(txnState, key, computer)
		assert.NoError(t, err)
		assert.Equal(t, value, val)
		assert.False(t, computer.called)

		_, found := view.Finalize().ReadSet[key]
		assert.True(t, found)
	})
}

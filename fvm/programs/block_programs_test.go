package programs

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
)

func TestBlockProgsWithTransactionOffset(t *testing.T) {
	block := NewEmptyDerivedBlockDataWithTransactionOffset(18)

	require.Equal(
		t,
		LogicalTime(17),
		block.LatestCommitExecutionTimeForTestingOnly())
}

func TestTxnProgsNormalTransactionInvalidExecutionTimeBound(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	_, err := block.NewDerivedTransactionData(-1, -1)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	_, err = block.NewDerivedTransactionData(0, EndOfBlockExecutionTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewDerivedTransactionData(0, EndOfBlockExecutionTime-1)
	require.NoError(t, err)
}

func TestTxnProgsNormalTransactionInvalidSnapshotTime(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	_, err := block.NewDerivedTransactionData(10, 0)
	require.ErrorContains(t, err, "snapshot > execution")

	_, err = block.NewDerivedTransactionData(10, 10)
	require.NoError(t, err)

	_, err = block.NewDerivedTransactionData(999, 998)
	require.ErrorContains(t, err, "snapshot > execution")

	_, err = block.NewDerivedTransactionData(999, 999)
	require.NoError(t, err)
}

func TestTxnProgsSnapshotReadTransactionInvalidExecutionTimeBound(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	_, err := block.NewSnapshotReadDerivedTransactionData(
		ParentBlockTime,
		ParentBlockTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewSnapshotReadDerivedTransactionData(ParentBlockTime, 0)
	require.NoError(t, err)

	_, err = block.NewSnapshotReadDerivedTransactionData(0, ChildBlockTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewSnapshotReadDerivedTransactionData(
		0,
		EndOfBlockExecutionTime)
	require.NoError(t, err)
}

func TestTxnProgsValidateRejectOutOfOrderCommit(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testTxn, err := block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	testSetupTxn, err := block.NewDerivedTransactionData(0, 1)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.NoError(t, validateErr)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	validateErr = testTxn.Validate()
	require.ErrorContains(t, validateErr, "non-increasing time")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectNonIncreasingExecutionTime(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testSetupTxn, err := block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "non-increasing time")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectCommitGapForNormalTxn(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewDerivedTransactionData(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewDerivedTransactionData(10, 10)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "missing commit range [6, 10)")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectCommitGapForSnapshotRead(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewDerivedTransactionData(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewSnapshotReadDerivedTransactionData(10, 10)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "missing commit range [6, 10)")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectOutdatedReadSet(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testSetupTxn1, err := block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	testSetupTxn2, err := block.NewDerivedTransactionData(0, 1)
	require.NoError(t, err)

	testTxn, err := block.NewDerivedTransactionData(0, 2)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}
	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testSetupTxn1.SetProgram(location, expectedProg, expectedState)

	testSetupTxn1.AddInvalidator(testInvalidator{})

	err = testSetupTxn1.Commit()
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.NoError(t, validateErr)

	actualProg, actualState, ok := testTxn.GetProgram(location)
	require.True(t, ok)
	require.Same(t, expectedProg, actualProg)
	require.Same(t, expectedState, actualState)

	validateErr = testTxn.Validate()
	require.NoError(t, validateErr)

	testSetupTxn2.AddInvalidator(testInvalidator{invalidateAll: true})

	err = testSetupTxn2.Commit()
	require.NoError(t, err)

	validateErr = testTxn.Validate()
	require.ErrorContains(t, validateErr, "outdated read set")
	require.True(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectOutdatedWriteSet(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testSetupTxn, err := block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	testSetupTxn.AddInvalidator(testInvalidator{invalidateAll: true})

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, 1, len(block.InvalidatorsForTestingOnly()))

	testTxn, err := block.NewDerivedTransactionData(0, 1)
	require.NoError(t, err)

	testTxn.SetProgram(
		common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{2, 3, 4}),
			Name:    "address",
		},
		&interpreter.Program{},
		&state.State{})

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "outdated write set")
	require.True(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateIgnoreInvalidatorsOlderThanSnapshot(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testSetupTxn, err := block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	testSetupTxn.AddInvalidator(testInvalidator{invalidateAll: true})
	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, 1, len(block.InvalidatorsForTestingOnly()))

	testTxn, err := block.NewDerivedTransactionData(1, 1)
	require.NoError(t, err)

	testTxn.SetProgram(
		common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{2, 3, 4}),
			Name:    "address",
		},
		&interpreter.Program{},
		&state.State{})

	err = testTxn.Validate()
	require.NoError(t, err)
}

func TestTxnProgsCommitEndOfBlockSnapshotRead(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewDerivedTransactionData(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewSnapshotReadDerivedTransactionData(
		EndOfBlockExecutionTime,
		EndOfBlockExecutionTime)
	require.NoError(t, err)

	err = testTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())
}

func TestTxnProgsCommitSnapshotReadDontAdvanceTime(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	commitTime := LogicalTime(71)
	testSetupTxn, err := block.NewDerivedTransactionData(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	repeatedTime := commitTime + 1
	for i := 0; i < 10; i++ {
		txn, err := block.NewSnapshotReadDerivedTransactionData(0, repeatedTime)
		require.NoError(t, err)

		err = txn.Commit()
		require.NoError(t, err)
	}

	require.Equal(
		t,
		commitTime,
		block.LatestCommitExecutionTimeForTestingOnly())
}

func TestTxnProgsCommitWriteOnlyTransactionNoInvalidation(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testTxn, err := block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}

	actualProg, actualState, ok := testTxn.GetProgram(location)
	require.False(t, ok)
	require.Nil(t, actualProg)
	require.Nil(t, actualState)

	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testTxn.SetProgram(location, expectedProg, expectedState)

	actualProg, actualState, ok = testTxn.GetProgram(location)
	require.True(t, ok)
	require.Same(t, expectedProg, actualProg)
	require.Same(t, expectedState, actualState)

	testTxn.AddInvalidator(testInvalidator{})

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

	entry, ok := entries[location]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, expectedProg, entry.Value)
	require.Same(t, expectedState, entry.State)
}

func TestTxnProgsCommitWriteOnlyTransactionWithInvalidation(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testTxnTime := LogicalTime(47)
	testTxn, err := block.NewDerivedTransactionData(0, testTxnTime)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}

	actualProg, actualState, ok := testTxn.GetProgram(location)
	require.False(t, ok)
	require.Nil(t, actualProg)
	require.Nil(t, actualState)

	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testTxn.SetProgram(location, expectedProg, expectedState)

	actualProg, actualState, ok = testTxn.GetProgram(location)
	require.True(t, ok)
	require.Same(t, expectedProg, actualProg)
	require.Same(t, expectedState, actualState)

	invalidator := testInvalidator{invalidateAll: true}

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
		chainedDerivedDataInvalidators[
			common.AddressLocation,
			*interpreter.Program,
		]{
			{
				DerivedDataInvalidator: invalidator,
				executionTime:          testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	require.Equal(t, 0, len(block.EntriesForTestingOnly()))
}

func TestTxnProgsCommitUseOriginalEntryOnDuplicateWriteEntries(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testSetupTxn, err := block.NewDerivedTransactionData(0, 11)
	require.NoError(t, err)

	testTxn, err := block.NewDerivedTransactionData(10, 12)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}
	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testSetupTxn.SetProgram(location, expectedProg, expectedState)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 1, len(entries))

	expectedEntry, ok := entries[location]
	require.True(t, ok)

	otherProg := &interpreter.Program{}
	otherState := &state.State{}

	testTxn.SetProgram(location, otherProg, otherState)

	err = testTxn.Commit()
	require.NoError(t, err)

	entries = block.EntriesForTestingOnly()
	require.Equal(t, 1, len(entries))

	actualEntry, ok := entries[location]
	require.True(t, ok)

	require.Same(t, expectedEntry, actualEntry)
	require.False(t, actualEntry.isInvalid)
	require.Same(t, expectedProg, actualEntry.Value)
	require.Same(t, expectedState, actualEntry.State)
	require.NotSame(t, otherProg, actualEntry.Value)
	require.NotSame(t, otherState, actualEntry.State)
}

func TestTxnProgsCommitReadOnlyTransactionNoInvalidation(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testSetupTxn, err := block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	testTxn, err := block.NewDerivedTransactionData(0, 1)
	require.NoError(t, err)

	loc1 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address1",
	}
	expectedProg1 := &interpreter.Program{}
	expectedState1 := &state.State{}

	testSetupTxn.SetProgram(loc1, expectedProg1, expectedState1)

	loc2 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address2",
	}
	expectedProg2 := &interpreter.Program{}
	expectedState2 := &state.State{}

	testSetupTxn.SetProgram(loc2, expectedProg2, expectedState2)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	actualProg, actualState, ok := testTxn.GetProgram(loc1)
	require.True(t, ok)
	require.Same(t, expectedProg1, actualProg)
	require.Same(t, expectedState1, actualState)

	actualProg, actualState, ok = testTxn.GetProgram(loc2)
	require.True(t, ok)
	require.Same(t, expectedProg2, actualProg)
	require.Same(t, expectedState2, actualState)

	actualProg, actualState, ok = testTxn.GetProgram(
		common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{2, 3, 4}),
			Name:    "address3",
		})
	require.False(t, ok)
	require.Nil(t, actualProg)
	require.Nil(t, actualState)

	testTxn.AddInvalidator(testInvalidator{})

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

	entry, ok := entries[loc1]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, expectedProg1, entry.Value)
	require.Same(t, expectedState1, entry.State)

	entry, ok = entries[loc2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, expectedProg2, entry.Value)
	require.Same(t, expectedState2, entry.State)
}

func TestTxnProgsCommitReadOnlyTransactionWithInvalidation(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testSetupTxn1Time := LogicalTime(2)
	testSetupTxn1, err := block.NewDerivedTransactionData(0, testSetupTxn1Time)
	require.NoError(t, err)

	testSetupTxn2, err := block.NewDerivedTransactionData(0, 4)
	require.NoError(t, err)

	testTxnTime := LogicalTime(6)
	testTxn, err := block.NewDerivedTransactionData(0, testTxnTime)
	require.NoError(t, err)

	testSetupTxn1Invalidator := testInvalidator{
		invalidateName: "blah",
	}
	testSetupTxn1.AddInvalidator(testSetupTxn1Invalidator)

	err = testSetupTxn1.Commit()
	require.NoError(t, err)

	loc1 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address1",
	}
	expectedProg1 := &interpreter.Program{}
	expectedState1 := &state.State{}

	testSetupTxn2.SetProgram(loc1, expectedProg1, expectedState1)

	loc2 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address2",
	}
	expectedProg2 := &interpreter.Program{}
	expectedState2 := &state.State{}

	testSetupTxn2.SetProgram(loc2, expectedProg2, expectedState2)

	err = testSetupTxn2.Commit()
	require.NoError(t, err)

	actualProg, actualState, ok := testTxn.GetProgram(loc1)
	require.True(t, ok)
	require.Same(t, expectedProg1, actualProg)
	require.Same(t, expectedState1, actualState)

	actualProg, actualState, ok = testTxn.GetProgram(loc2)
	require.True(t, ok)
	require.Same(t, expectedProg2, actualProg)
	require.Same(t, expectedState2, actualState)

	actualProg, actualState, ok = testTxn.GetProgram(
		common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{2, 3, 4}),
			Name:    "address3",
		})
	require.False(t, ok)
	require.Nil(t, actualProg)
	require.Nil(t, actualState)

	testTxnInvalidator := testInvalidator{invalidateAll: true}
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
		chainedDerivedDataInvalidators[
			common.AddressLocation,
			*interpreter.Program,
		]{
			{
				DerivedDataInvalidator: testSetupTxn1Invalidator,
				executionTime:          testSetupTxn1Time,
			},
			{
				DerivedDataInvalidator: testTxnInvalidator,
				executionTime:          testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	require.Equal(t, 0, len(block.EntriesForTestingOnly()))
}

func TestTxnProgsCommitValidateError(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testSetupTxn, err := block.NewDerivedTransactionData(0, 10)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewDerivedTransactionData(10, 10)
	require.NoError(t, err)

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "non-increasing time")
	require.False(t, commitErr.IsRetryable())
}

func TestTxnProgsCommitSnapshotReadDoesNotAdvanceCommitTime(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	expectedTime := LogicalTime(10)
	testSetupTxn, err := block.NewDerivedTransactionData(0, expectedTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewSnapshotReadDerivedTransactionData(0, 11)
	require.NoError(t, err)

	err = testTxn.Commit()
	require.NoError(t, err)

	require.Equal(
		t,
		expectedTime,
		block.LatestCommitExecutionTimeForTestingOnly())
}

func TestTxnProgsCommitBadSnapshotReadInvalidator(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	testTxn, err := block.NewSnapshotReadDerivedTransactionData(0, 42)
	require.NoError(t, err)

	testTxn.AddInvalidator(testInvalidator{invalidateAll: true})

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "snapshot read can't invalidate")
	require.False(t, commitErr.IsRetryable())
}

func TestTxnProgsCommitFineGrainInvalidation(t *testing.T) {
	block := NewEmptyDerivedBlockData()

	// Setup the database with two read entries

	testSetupTxn, err := block.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	readLoc1Name := "read-address1"
	readLoc1 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    readLoc1Name,
	}
	readProg1 := &interpreter.Program{}
	readState1 := &state.State{}

	readLoc2 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "read-address2",
	}
	readProg2 := &interpreter.Program{}
	readState2 := &state.State{}

	testSetupTxn.SetProgram(readLoc1, readProg1, readState1)
	testSetupTxn.SetProgram(readLoc2, readProg2, readState2)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	// Setup the test transaction by read both existing entries and writing
	// two new ones,

	testTxnTime := LogicalTime(15)
	testTxn, err := block.NewDerivedTransactionData(1, testTxnTime)
	require.NoError(t, err)

	actualProg, actualState, ok := testTxn.GetProgram(readLoc1)
	require.True(t, ok)
	require.Same(t, readProg1, actualProg)
	require.Same(t, readState1, actualState)

	actualProg, actualState, ok = testTxn.GetProgram(readLoc2)
	require.True(t, ok)
	require.Same(t, readProg2, actualProg)
	require.Same(t, readState2, actualState)

	writeLoc1Name := "write-address1"
	writeLoc1 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    writeLoc1Name,
	}
	writeProg1 := &interpreter.Program{}
	writeState1 := &state.State{}

	writeLoc2 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "write-address2",
	}
	writeProg2 := &interpreter.Program{}
	writeState2 := &state.State{}

	testTxn.SetProgram(writeLoc1, writeProg1, writeState1)
	testTxn.SetProgram(writeLoc2, writeProg2, writeState2)

	// Actual test.  Invalidate one pre-existing entry and one new entry.

	invalidator1 := testInvalidator{
		invalidateName: readLoc1Name,
	}
	invalidator2 := testInvalidator{
		invalidateName: writeLoc1Name,
	}
	testTxn.AddInvalidator(nil)
	testTxn.AddInvalidator(invalidator1)
	testTxn.AddInvalidator(testInvalidator{})
	testTxn.AddInvalidator(invalidator2)
	testTxn.AddInvalidator(testInvalidator{})

	err = testTxn.Commit()
	require.NoError(t, err)

	require.Equal(
		t,
		testTxnTime,
		block.LatestCommitExecutionTimeForTestingOnly())

	require.Equal(
		t,
		chainedDerivedDataInvalidators[
			common.AddressLocation,
			*interpreter.Program,
		]{
			{
				DerivedDataInvalidator: invalidator1,
				executionTime:          testTxnTime,
			},
			{
				DerivedDataInvalidator: invalidator2,
				executionTime:          testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 2, len(entries))

	entry, ok := entries[readLoc2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, readProg2, entry.Value)
	require.Same(t, readState2, entry.State)

	entry, ok = entries[writeLoc2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Same(t, writeProg2, entry.Value)
	require.Same(t, writeState2, entry.State)
}

func TestBlockProgsNewChildDerivedBlockData(t *testing.T) {
	parentBlock := NewEmptyDerivedBlockData()

	require.Equal(
		t,
		ParentBlockTime,
		parentBlock.LatestCommitExecutionTimeForTestingOnly())
	require.Equal(t, 0, len(parentBlock.InvalidatorsForTestingOnly()))
	require.Equal(t, 0, len(parentBlock.EntriesForTestingOnly()))

	txn, err := parentBlock.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	txn.AddInvalidator(testInvalidator{invalidateAll: true})

	err = txn.Commit()
	require.NoError(t, err)

	txn, err = parentBlock.NewDerivedTransactionData(1, 1)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}
	prog := &interpreter.Program{}
	state := &state.State{}

	txn.SetProgram(location, prog, state)

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

	parentEntry, ok := parentEntries[location]
	require.True(t, ok)
	require.False(t, parentEntry.isInvalid)
	require.Same(t, prog, parentEntry.Value)
	require.Same(t, state, parentEntry.State)

	// Verify child is correctly initialized

	childBlock := parentBlock.NewChildDerivedBlockData()

	require.Equal(
		t,
		ParentBlockTime,
		childBlock.LatestCommitExecutionTimeForTestingOnly())

	require.Equal(t, 0, len(childBlock.InvalidatorsForTestingOnly()))

	childEntries := childBlock.EntriesForTestingOnly()
	require.Equal(t, 1, len(childEntries))

	childEntry, ok := childEntries[location]
	require.True(t, ok)
	require.False(t, childEntry.isInvalid)
	require.Same(t, prog, childEntry.Value)
	require.Same(t, state, childEntry.State)

	require.NotSame(t, parentEntry, childEntry)
}

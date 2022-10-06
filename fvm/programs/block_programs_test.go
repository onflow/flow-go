package programs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
)

func TestBlockProgsWithTransactionOffset(t *testing.T) {
	block := NewEmptyBlockProgramsWithTransactionOffset(18)

	require.Equal(
		t,
		LogicalTime(17),
		block.LatestCommitExecutionTimeForTestingOnly())
}

func TestTxnProgsNormalTransactionInvalidExecutionTimeBound(t *testing.T) {
	block := NewEmptyBlockPrograms()

	_, err := block.NewTransactionPrograms(-1, -1)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	_, err = block.NewTransactionPrograms(0, EndOfBlockExecutionTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewTransactionPrograms(0, EndOfBlockExecutionTime-1)
	require.NoError(t, err)
}

func TestTxnProgsNormalTransactionInvalidSnapshotTime(t *testing.T) {
	block := NewEmptyBlockPrograms()

	_, err := block.NewTransactionPrograms(10, 0)
	require.ErrorContains(t, err, "snapshot > execution")

	_, err = block.NewTransactionPrograms(10, 10)
	require.NoError(t, err)

	_, err = block.NewTransactionPrograms(999, 998)
	require.ErrorContains(t, err, "snapshot > execution")

	_, err = block.NewTransactionPrograms(999, 999)
	require.NoError(t, err)
}

func TestTxnProgsSnapshotReadTransactionInvalidExecutionTimeBound(t *testing.T) {
	block := NewEmptyBlockPrograms()

	_, err := block.NewSnapshotReadTransactionPrograms(
		ParentBlockTime,
		ParentBlockTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewSnapshotReadTransactionPrograms(ParentBlockTime, 0)
	require.NoError(t, err)

	_, err = block.NewSnapshotReadTransactionPrograms(0, ChildBlockTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewSnapshotReadTransactionPrograms(
		0,
		EndOfBlockExecutionTime)
	require.NoError(t, err)
}

func TestTxnProgsValidateRejectOutOfOrderCommit(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testTxn, err := block.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	testSetupTxn, err := block.NewTransactionPrograms(0, 1)
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
	block := NewEmptyBlockPrograms()

	testSetupTxn, err := block.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "non-increasing time")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectCommitGapForNormalTxn(t *testing.T) {
	block := NewEmptyBlockPrograms()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewTransactionPrograms(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewTransactionPrograms(10, 10)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "missing commit range [6, 10)")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectCommitGapForSnapshotRead(t *testing.T) {
	block := NewEmptyBlockPrograms()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewTransactionPrograms(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewSnapshotReadTransactionPrograms(10, 10)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "missing commit range [6, 10)")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectOutdatedReadSet(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn1, err := block.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	testSetupTxn2, err := block.NewTransactionPrograms(0, 1)
	require.NoError(t, err)

	testTxn, err := block.NewTransactionPrograms(0, 2)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}
	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testSetupTxn1.Set(location, expectedProg, expectedState)

	testSetupTxn1.AddInvalidator(testInvalidator{})

	err = testSetupTxn1.Commit()
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.NoError(t, validateErr)

	actualProg, actualState, ok := testTxn.Get(location)
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
	block := NewEmptyBlockPrograms()

	testSetupTxn, err := block.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	testSetupTxn.AddInvalidator(testInvalidator{invalidateAll: true})

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, 1, len(block.InvalidatorsForTestingOnly()))

	testTxn, err := block.NewTransactionPrograms(0, 1)
	require.NoError(t, err)

	testTxn.Set(
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
	block := NewEmptyBlockPrograms()

	testSetupTxn, err := block.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	testSetupTxn.AddInvalidator(testInvalidator{invalidateAll: true})
	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, 1, len(block.InvalidatorsForTestingOnly()))

	testTxn, err := block.NewTransactionPrograms(1, 1)
	require.NoError(t, err)

	testTxn.Set(
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
	block := NewEmptyBlockPrograms()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewTransactionPrograms(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewSnapshotReadTransactionPrograms(
		EndOfBlockExecutionTime,
		EndOfBlockExecutionTime)
	require.NoError(t, err)

	err = testTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())
}

func TestTxnProgsCommitSnapshotReadDontAdvanceTime(t *testing.T) {
	block := NewEmptyBlockPrograms()

	commitTime := LogicalTime(71)
	testSetupTxn, err := block.NewTransactionPrograms(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	repeatedTime := commitTime + 1
	for i := 0; i < 10; i++ {
		txn, err := block.NewSnapshotReadTransactionPrograms(0, repeatedTime)
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
	block := NewEmptyBlockPrograms()

	testTxn, err := block.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}

	actualProg, actualState, ok := testTxn.Get(location)
	require.False(t, ok)
	require.Nil(t, actualProg)
	require.Nil(t, actualState)

	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testTxn.Set(location, expectedProg, expectedState)

	actualProg, actualState, ok = testTxn.Get(location)
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
	require.Equal(t, location, entry.Location)
	require.Same(t, expectedProg, entry.Program)
	require.Same(t, expectedState, entry.State)
}

func TestTxnProgsCommitWriteOnlyTransactionWithInvalidation(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testTxnTime := LogicalTime(47)
	testTxn, err := block.NewTransactionPrograms(0, testTxnTime)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}

	actualProg, actualState, ok := testTxn.Get(location)
	require.False(t, ok)
	require.Nil(t, actualProg)
	require.Nil(t, actualState)

	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testTxn.Set(location, expectedProg, expectedState)

	actualProg, actualState, ok = testTxn.Get(location)
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
		chainedInvalidators{
			{
				Invalidator:   invalidator,
				executionTime: testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	require.Equal(t, 0, len(block.EntriesForTestingOnly()))
}

func TestTxnProgsCommitUseOriginalEntryOnDuplicateWriteEntries(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn, err := block.NewTransactionPrograms(0, 11)
	require.NoError(t, err)

	testTxn, err := block.NewTransactionPrograms(10, 12)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}
	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testSetupTxn.Set(location, expectedProg, expectedState)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 1, len(entries))

	expectedEntry, ok := entries[location]
	require.True(t, ok)

	otherProg := &interpreter.Program{}
	otherState := &state.State{}

	testTxn.Set(location, otherProg, otherState)

	err = testTxn.Commit()
	require.NoError(t, err)

	entries = block.EntriesForTestingOnly()
	require.Equal(t, 1, len(entries))

	actualEntry, ok := entries[location]
	require.True(t, ok)

	require.Same(t, expectedEntry, actualEntry)
	require.False(t, actualEntry.isInvalid)
	require.Equal(t, location, actualEntry.Location)
	require.Same(t, expectedProg, actualEntry.Program)
	require.Same(t, expectedState, actualEntry.State)
	require.NotSame(t, otherProg, actualEntry.Program)
	require.NotSame(t, otherState, actualEntry.State)
}

func TestTxnProgsCommitReadOnlyTransactionNoInvalidation(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn, err := block.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	testTxn, err := block.NewTransactionPrograms(0, 1)
	require.NoError(t, err)

	loc1 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address1",
	}
	expectedProg1 := &interpreter.Program{}
	expectedState1 := &state.State{}

	testSetupTxn.Set(loc1, expectedProg1, expectedState1)

	loc2 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address2",
	}
	expectedProg2 := &interpreter.Program{}
	expectedState2 := &state.State{}

	testSetupTxn.Set(loc2, expectedProg2, expectedState2)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	actualProg, actualState, ok := testTxn.Get(loc1)
	require.True(t, ok)
	require.Same(t, expectedProg1, actualProg)
	require.Same(t, expectedState1, actualState)

	actualProg, actualState, ok = testTxn.Get(loc2)
	require.True(t, ok)
	require.Same(t, expectedProg2, actualProg)
	require.Same(t, expectedState2, actualState)

	actualProg, actualState, ok = testTxn.Get(
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
	require.Equal(t, loc1, entry.Location)
	require.Same(t, expectedProg1, entry.Program)
	require.Same(t, expectedState1, entry.State)

	entry, ok = entries[loc2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Equal(t, loc2, entry.Location)
	require.Same(t, expectedProg2, entry.Program)
	require.Same(t, expectedState2, entry.State)
}

func TestTxnProgsCommitReadOnlyTransactionWithInvalidation(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn1Time := LogicalTime(2)
	testSetupTxn1, err := block.NewTransactionPrograms(0, testSetupTxn1Time)
	require.NoError(t, err)

	testSetupTxn2, err := block.NewTransactionPrograms(0, 4)
	require.NoError(t, err)

	testTxnTime := LogicalTime(6)
	testTxn, err := block.NewTransactionPrograms(0, testTxnTime)
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

	testSetupTxn2.Set(loc1, expectedProg1, expectedState1)

	loc2 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address2",
	}
	expectedProg2 := &interpreter.Program{}
	expectedState2 := &state.State{}

	testSetupTxn2.Set(loc2, expectedProg2, expectedState2)

	err = testSetupTxn2.Commit()
	require.NoError(t, err)

	actualProg, actualState, ok := testTxn.Get(loc1)
	require.True(t, ok)
	require.Same(t, expectedProg1, actualProg)
	require.Same(t, expectedState1, actualState)

	actualProg, actualState, ok = testTxn.Get(loc2)
	require.True(t, ok)
	require.Same(t, expectedProg2, actualProg)
	require.Same(t, expectedState2, actualState)

	actualProg, actualState, ok = testTxn.Get(
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
		chainedInvalidators{
			{
				Invalidator:   testSetupTxn1Invalidator,
				executionTime: testSetupTxn1Time,
			},
			{
				Invalidator:   testTxnInvalidator,
				executionTime: testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	require.Equal(t, 0, len(block.EntriesForTestingOnly()))
}

func TestTxnProgsCommitNonAddressPrograms(t *testing.T) {
	block := NewEmptyBlockPrograms()

	expectedTime := LogicalTime(121)
	testTxn, err := block.NewTransactionPrograms(0, expectedTime)
	require.NoError(t, err)

	location := common.IdentifierLocation("non-address")

	actualProg, actualState, ok := testTxn.Get(location)
	require.False(t, ok)
	require.Nil(t, actualProg)
	require.Nil(t, actualState)

	expectedProg := &interpreter.Program{}

	testTxn.Set(location, expectedProg, nil)

	actualProg, actualState, ok = testTxn.Get(location)
	require.True(t, ok)
	require.Same(t, expectedProg, actualProg)
	require.Nil(t, actualState)

	err = testTxn.Commit()
	require.NoError(t, err)

	// Sanity Check

	require.Equal(
		t,
		expectedTime,
		block.LatestCommitExecutionTimeForTestingOnly())
	require.Equal(t, 0, len(block.InvalidatorsForTestingOnly()))
	require.Equal(t, 0, len(block.EntriesForTestingOnly()))
}

func TestTxnProgsCommitValidateError(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn, err := block.NewTransactionPrograms(0, 10)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewTransactionPrograms(10, 10)
	require.NoError(t, err)

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "non-increasing time")
	require.False(t, commitErr.IsRetryable())
}

func TestTxnProgsCommitSnapshotReadDoesNotAdvanceCommitTime(t *testing.T) {
	block := NewEmptyBlockPrograms()

	expectedTime := LogicalTime(10)
	testSetupTxn, err := block.NewTransactionPrograms(0, expectedTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewSnapshotReadTransactionPrograms(0, 11)
	require.NoError(t, err)

	err = testTxn.Commit()
	require.NoError(t, err)

	require.Equal(
		t,
		expectedTime,
		block.LatestCommitExecutionTimeForTestingOnly())
}

func TestTxnProgsCommitBadSnapshotReadInvalidator(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testTxn, err := block.NewSnapshotReadTransactionPrograms(0, 42)
	require.NoError(t, err)

	testTxn.AddInvalidator(testInvalidator{invalidateAll: true})

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "snapshot read can't invalidate")
	require.False(t, commitErr.IsRetryable())
}

func TestTxnProgsCommitFineGrainInvalidation(t *testing.T) {
	block := NewEmptyBlockPrograms()

	// Setup the database with two read entries

	testSetupTxn, err := block.NewTransactionPrograms(0, 0)
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

	testSetupTxn.Set(readLoc1, readProg1, readState1)
	testSetupTxn.Set(readLoc2, readProg2, readState2)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	// Setup the test transaction by read both existing entries and writing
	// two new ones,

	testTxnTime := LogicalTime(15)
	testTxn, err := block.NewTransactionPrograms(1, testTxnTime)
	require.NoError(t, err)

	actualProg, actualState, ok := testTxn.Get(readLoc1)
	require.True(t, ok)
	require.Same(t, readProg1, actualProg)
	require.Same(t, readState1, actualState)

	actualProg, actualState, ok = testTxn.Get(readLoc2)
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

	testTxn.Set(writeLoc1, writeProg1, writeState1)
	testTxn.Set(writeLoc2, writeProg2, writeState2)

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
		chainedInvalidators{
			{
				Invalidator:   invalidator1,
				executionTime: testTxnTime,
			},
			{
				Invalidator:   invalidator2,
				executionTime: testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 2, len(entries))

	entry, ok := entries[readLoc2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Equal(t, readLoc2, entry.Location)
	require.Same(t, readProg2, entry.Program)
	require.Same(t, readState2, entry.State)

	entry, ok = entries[writeLoc2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Equal(t, writeLoc2, entry.Location)
	require.Same(t, writeProg2, entry.Program)
	require.Same(t, writeState2, entry.State)
}

func TestBlockProgsNewChildBlockPrograms(t *testing.T) {
	parentBlock := NewEmptyBlockPrograms()

	require.Equal(
		t,
		ParentBlockTime,
		parentBlock.LatestCommitExecutionTimeForTestingOnly())
	require.Equal(t, 0, len(parentBlock.InvalidatorsForTestingOnly()))
	require.Equal(t, 0, len(parentBlock.EntriesForTestingOnly()))

	txn, err := parentBlock.NewTransactionPrograms(0, 0)
	require.NoError(t, err)

	txn.AddInvalidator(testInvalidator{invalidateAll: true})

	err = txn.Commit()
	require.NoError(t, err)

	txn, err = parentBlock.NewTransactionPrograms(1, 1)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}
	prog := &interpreter.Program{}
	state := &state.State{}

	txn.Set(location, prog, state)

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
	require.Equal(t, location, parentEntry.Location)
	require.Same(t, prog, parentEntry.Program)
	require.Same(t, state, parentEntry.State)

	// Verify child is correctly initialized

	childBlock := parentBlock.NewChildBlockPrograms()

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
	require.Equal(t, location, childEntry.Location)
	require.Same(t, prog, childEntry.Program)
	require.Same(t, state, childEntry.State)

	require.NotSame(t, parentEntry, childEntry)
}

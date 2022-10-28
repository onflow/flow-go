package programs

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/require"

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

	_, err := block.NewOCCBlockItem(-1, -1)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	_, err = block.NewOCCBlockItem(0, EndOfBlockExecutionTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewOCCBlockItem(0, EndOfBlockExecutionTime-1)
	require.NoError(t, err)
}

func TestTxnProgsNormalTransactionInvalidSnapshotTime(t *testing.T) {
	block := NewEmptyBlockPrograms()

	_, err := block.NewOCCBlockItem(10, 0)
	require.ErrorContains(t, err, "snapshot > execution")

	_, err = block.NewOCCBlockItem(10, 10)
	require.NoError(t, err)

	_, err = block.NewOCCBlockItem(999, 998)
	require.ErrorContains(t, err, "snapshot > execution")

	_, err = block.NewOCCBlockItem(999, 999)
	require.NoError(t, err)
}

func TestTxnProgsSnapshotReadTransactionInvalidExecutionTimeBound(t *testing.T) {
	block := NewEmptyBlockPrograms()

	_, err := block.NewSnapshotReadOCCBlockItem(
		ParentBlockTime,
		ParentBlockTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewSnapshotReadOCCBlockItem(ParentBlockTime, 0)
	require.NoError(t, err)

	_, err = block.NewSnapshotReadOCCBlockItem(0, ChildBlockTime)
	require.ErrorContains(t, err, "execution time out of bound")

	_, err = block.NewSnapshotReadOCCBlockItem(
		0,
		EndOfBlockExecutionTime)
	require.NoError(t, err)
}

func TestTxnProgsValidateRejectOutOfOrderCommit(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testTxn, err := block.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	testSetupTxn, err := block.NewOCCBlockItem(0, 1)
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

	testSetupTxn, err := block.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "non-increasing time")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectCommitGapForNormalTxn(t *testing.T) {
	block := NewEmptyBlockPrograms()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewOCCBlockItem(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewOCCBlockItem(10, 10)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "missing commit range [6, 10)")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectCommitGapForSnapshotRead(t *testing.T) {
	block := NewEmptyBlockPrograms()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewOCCBlockItem(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewSnapshotReadOCCBlockItem(10, 10)
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "missing commit range [6, 10)")
	require.False(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateRejectOutdatedReadSet(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn1, err := block.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	testSetupTxn2, err := block.NewOCCBlockItem(0, 1)
	require.NoError(t, err)

	testTxn, err := block.NewOCCBlockItem(0, 2)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}
	expectedProgramEntry := &ProgramEntry{
		Program: &interpreter.Program{},
		State:   &state.State{},
	}

	testSetupTxn1.Set(location, *expectedProgramEntry)

	testSetupTxn1.AddInvalidator(testInvalidator{})

	err = testSetupTxn1.Commit()
	require.NoError(t, err)

	validateErr := testTxn.Validate()
	require.NoError(t, validateErr)

	actualProgramEntry := testTxn.Get(location)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, expectedProgramEntry.Program, actualProgramEntry.Program)
	require.Same(t, expectedProgramEntry.State, actualProgramEntry.State)

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

	testSetupTxn, err := block.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	testSetupTxn.AddInvalidator(testInvalidator{invalidateAll: true})

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, 1, len(block.InvalidatorsForTestingOnly()))

	testTxn, err := block.NewOCCBlockItem(0, 1)
	require.NoError(t, err)

	testTxn.Set(
		common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{2, 3, 4}),
			Name:    "address",
		},
		ProgramEntry{
			Program: &interpreter.Program{},
			State:   &state.State{},
		})

	validateErr := testTxn.Validate()
	require.ErrorContains(t, validateErr, "outdated write set")
	require.True(t, validateErr.IsRetryable())
}

func TestTxnProgsValidateIgnoreInvalidatorsOlderThanSnapshot(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn, err := block.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	testSetupTxn.AddInvalidator(testInvalidator{invalidateAll: true})
	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, 1, len(block.InvalidatorsForTestingOnly()))

	testTxn, err := block.NewOCCBlockItem(1, 1)
	require.NoError(t, err)

	testTxn.Set(
		common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{2, 3, 4}),
			Name:    "address",
		},
		ProgramEntry{
			Program: &interpreter.Program{},
			State:   &state.State{},
		})

	err = testTxn.Validate()
	require.NoError(t, err)
}

func TestTxnProgsCommitEndOfBlockSnapshotRead(t *testing.T) {
	block := NewEmptyBlockPrograms()

	commitTime := LogicalTime(5)
	testSetupTxn, err := block.NewOCCBlockItem(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, commitTime, block.LatestCommitExecutionTimeForTestingOnly())

	testTxn, err := block.NewSnapshotReadOCCBlockItem(
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
	testSetupTxn, err := block.NewOCCBlockItem(0, commitTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	repeatedTime := commitTime + 1
	for i := 0; i < 10; i++ {
		txn, err := block.NewSnapshotReadOCCBlockItem(0, repeatedTime)
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

	testTxn, err := block.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}

	programEntry := testTxn.Get(location)
	require.Nil(t, programEntry)

	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testTxn.Set(location, ProgramEntry{
		Location: location,
		Program:  expectedProg,
		State:    expectedState,
	})

	actualProgramEntry := testTxn.Get(location)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, expectedProg, actualProgramEntry.Program)
	require.Same(t, expectedState, actualProgramEntry.State)

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
	require.Equal(t, location, entry.Entry.Location)
	require.Same(t, expectedProg, entry.Entry.Program)
	require.Same(t, expectedState, entry.Entry.State)
}

func TestTxnProgsCommitWriteOnlyTransactionWithInvalidation(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testTxnTime := LogicalTime(47)
	testTxn, err := block.NewOCCBlockItem(0, testTxnTime)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}

	actualProgramEntry := testTxn.Get(location)
	require.Nil(t, actualProgramEntry)

	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testTxn.Set(location, ProgramEntry{
		Program: expectedProg,
		State:   expectedState,
	})

	actualProgramEntry = testTxn.Get(location)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, expectedProg, actualProgramEntry.Program)
	require.Same(t, expectedState, actualProgramEntry.State)

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
		chainedOCCInvalidators[ProgramEntry]{
			{
				OCCInvalidator: invalidator,
				executionTime:  testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	require.Equal(t, 0, len(block.EntriesForTestingOnly()))
}

func TestTxnProgsCommitUseOriginalEntryOnDuplicateWriteEntries(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn, err := block.NewOCCBlockItem(0, 11)
	require.NoError(t, err)

	testTxn, err := block.NewOCCBlockItem(10, 12)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}
	expectedProg := &interpreter.Program{}
	expectedState := &state.State{}

	testSetupTxn.Set(location, ProgramEntry{
		Location: location,
		Program:  expectedProg,
		State:    expectedState,
	})

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 1, len(entries))

	expectedEntry, ok := entries[location]
	require.True(t, ok)

	otherProg := &interpreter.Program{}
	otherState := &state.State{}

	testTxn.Set(location, ProgramEntry{
		Location: location,
		Program:  otherProg,
		State:    otherState,
	})

	err = testTxn.Commit()
	require.NoError(t, err)

	entries = block.EntriesForTestingOnly()
	require.Equal(t, 1, len(entries))

	actualEntry, ok := entries[location]
	require.True(t, ok)

	require.Same(t, expectedEntry, actualEntry)
	require.False(t, actualEntry.isInvalid)
	require.Equal(t, location, actualEntry.Entry.Location)
	require.Same(t, expectedProg, actualEntry.Entry.Program)
	require.Same(t, expectedState, actualEntry.Entry.State)
	require.NotSame(t, otherProg, actualEntry.Entry.Program)
	require.NotSame(t, otherState, actualEntry.Entry.State)
}

func TestTxnProgsCommitReadOnlyTransactionNoInvalidation(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn, err := block.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	testTxn, err := block.NewOCCBlockItem(0, 1)
	require.NoError(t, err)

	loc1 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address1",
	}
	expectedProg1 := &interpreter.Program{}
	expectedState1 := &state.State{}

	testSetupTxn.Set(loc1, ProgramEntry{
		Location: loc1,
		Program:  expectedProg1,
		State:    expectedState1,
	})

	loc2 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address2",
	}
	expectedProg2 := &interpreter.Program{}
	expectedState2 := &state.State{}

	testSetupTxn.Set(loc2, ProgramEntry{
		Location: loc2,
		Program:  expectedProg2,
		State:    expectedState2,
	})

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	actualProgramEntry := testTxn.Get(loc1)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, expectedProg1, actualProgramEntry.Program)
	require.Same(t, expectedState1, actualProgramEntry.State)

	actualProgramEntry = testTxn.Get(loc2)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, expectedProg2, actualProgramEntry.Program)
	require.Same(t, expectedState2, actualProgramEntry.State)

	actualProgramEntry = testTxn.Get(
		common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{2, 3, 4}),
			Name:    "address3",
		})
	require.Nil(t, actualProgramEntry)

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
	require.Equal(t, loc1, entry.Entry.Location)
	require.Same(t, expectedProg1, entry.Entry.Program)
	require.Same(t, expectedState1, entry.Entry.State)

	entry, ok = entries[loc2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Equal(t, loc2, entry.Entry.Location)
	require.Same(t, expectedProg2, entry.Entry.Program)
	require.Same(t, expectedState2, entry.Entry.State)
}

func TestTxnProgsCommitReadOnlyTransactionWithInvalidation(t *testing.T) {
	block := NewEmptyBlockPrograms()

	testSetupTxn1Time := LogicalTime(2)
	testSetupTxn1, err := block.NewOCCBlockItem(0, testSetupTxn1Time)
	require.NoError(t, err)

	testSetupTxn2, err := block.NewOCCBlockItem(0, 4)
	require.NoError(t, err)

	testTxnTime := LogicalTime(6)
	testTxn, err := block.NewOCCBlockItem(0, testTxnTime)
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

	testSetupTxn2.Set(loc1, ProgramEntry{
		Program: expectedProg1,
		State:   expectedState1,
	})

	loc2 := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address2",
	}
	expectedProg2 := &interpreter.Program{}
	expectedState2 := &state.State{}

	testSetupTxn2.Set(loc2, ProgramEntry{
		Program: expectedProg2,
		State:   expectedState2,
	})

	err = testSetupTxn2.Commit()
	require.NoError(t, err)

	actualProgramEntry := testTxn.Get(loc1)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, expectedProg1, actualProgramEntry.Program)
	require.Same(t, expectedState1, actualProgramEntry.State)

	actualProgramEntry = testTxn.Get(loc2)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, expectedProg2, actualProgramEntry.Program)
	require.Same(t, expectedState2, actualProgramEntry.State)

	actualProgramEntry = testTxn.Get(
		common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{2, 3, 4}),
			Name:    "address3",
		})
	require.Nil(t, actualProgramEntry)

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
		chainedOCCInvalidators[ProgramEntry]{
			{
				OCCInvalidator: testSetupTxn1Invalidator,
				executionTime:  testSetupTxn1Time,
			},
			{
				OCCInvalidator: testTxnInvalidator,
				executionTime:  testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	require.Equal(t, 0, len(block.EntriesForTestingOnly()))
}

func TestTxnProgsCommitNonAddressPrograms(t *testing.T) {
	block := NewEmptyBlockPrograms()

	expectedTime := LogicalTime(121)
	txn, err := block.NewOCCBlockItem(0, expectedTime)
	require.NoError(t, err)
	testTxn := &TransactionPrograms{
		transactionPrograms: *txn,
		nonAddressSet:       make(map[common.Location]ProgramEntry),
	}

	location := common.IdentifierLocation("non-address")

	actualProgramEntry := testTxn.Get(location)
	require.Nil(t, actualProgramEntry)

	expectedProg := &interpreter.Program{}

	testTxn.Set(location, ProgramEntry{
		Program: expectedProg,
		State:   nil,
	})

	actualProgramEntry = testTxn.Get(location)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, expectedProg, actualProgramEntry.Program)
	require.Nil(t, actualProgramEntry.State)

	err = testTxn.transactionPrograms.Commit()
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

	testSetupTxn, err := block.NewOCCBlockItem(0, 10)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewOCCBlockItem(10, 10)
	require.NoError(t, err)

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "non-increasing time")
	require.False(t, commitErr.IsRetryable())
}

func TestTxnProgsCommitSnapshotReadDoesNotAdvanceCommitTime(t *testing.T) {
	block := NewEmptyBlockPrograms()

	expectedTime := LogicalTime(10)
	testSetupTxn, err := block.NewOCCBlockItem(0, expectedTime)
	require.NoError(t, err)

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	testTxn, err := block.NewSnapshotReadOCCBlockItem(0, 11)
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

	testTxn, err := block.NewSnapshotReadOCCBlockItem(0, 42)
	require.NoError(t, err)

	testTxn.AddInvalidator(testInvalidator{invalidateAll: true})

	commitErr := testTxn.Commit()
	require.ErrorContains(t, commitErr, "snapshot read can't invalidate")
	require.False(t, commitErr.IsRetryable())
}

func TestTxnProgsCommitFineGrainInvalidation(t *testing.T) {
	block := NewEmptyBlockPrograms()

	// Setup the database with two read entries

	testSetupTxn, err := block.NewOCCBlockItem(0, 0)
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

	testSetupTxn.Set(readLoc1, ProgramEntry{
		Location: readLoc1,
		Program:  readProg1,
		State:    readState1,
	})
	testSetupTxn.Set(readLoc2, ProgramEntry{
		Location: readLoc2,
		Program:  readProg2,
		State:    readState2,
	})

	err = testSetupTxn.Commit()
	require.NoError(t, err)

	// Setup the test transaction by read both existing entries and writing
	// two new ones,

	testTxnTime := LogicalTime(15)
	testTxn, err := block.NewOCCBlockItem(1, testTxnTime)
	require.NoError(t, err)

	actualProgramEntry := testTxn.Get(readLoc1)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, readProg1, actualProgramEntry.Program)
	require.Same(t, readState1, actualProgramEntry.State)

	actualProgramEntry = testTxn.Get(readLoc2)
	require.NotNil(t, actualProgramEntry)
	require.Same(t, readProg2, actualProgramEntry.Program)
	require.Same(t, readState2, actualProgramEntry.State)

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

	testTxn.Set(writeLoc1, ProgramEntry{
		Location: writeLoc1,
		Program:  writeProg1,
		State:    writeState1,
	})
	testTxn.Set(writeLoc2, ProgramEntry{
		Location: writeLoc2,
		Program:  writeProg2,
		State:    writeState2,
	})

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
		chainedOCCInvalidators[ProgramEntry]{
			{
				OCCInvalidator: invalidator1,
				executionTime:  testTxnTime,
			},
			{
				OCCInvalidator: invalidator2,
				executionTime:  testTxnTime,
			},
		},
		block.InvalidatorsForTestingOnly())

	entries := block.EntriesForTestingOnly()
	require.Equal(t, 2, len(entries))

	entry, ok := entries[readLoc2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Equal(t, readLoc2, entry.Entry.Location)
	require.Same(t, readProg2, entry.Entry.Program)
	require.Same(t, readState2, entry.Entry.State)

	entry, ok = entries[writeLoc2]
	require.True(t, ok)
	require.False(t, entry.isInvalid)
	require.Equal(t, writeLoc2, entry.Entry.Location)
	require.Same(t, writeProg2, entry.Entry.Program)
	require.Same(t, writeState2, entry.Entry.State)
}

func TestBlockProgsNewChildBlockPrograms(t *testing.T) {
	parentBlock := NewEmptyBlockPrograms()

	require.Equal(
		t,
		ParentBlockTime,
		parentBlock.LatestCommitExecutionTimeForTestingOnly())
	require.Equal(t, 0, len(parentBlock.InvalidatorsForTestingOnly()))
	require.Equal(t, 0, len(parentBlock.EntriesForTestingOnly()))

	txn, err := parentBlock.NewOCCBlockItem(0, 0)
	require.NoError(t, err)

	txn.AddInvalidator(testInvalidator{invalidateAll: true})

	err = txn.Commit()
	require.NoError(t, err)

	txn, err = parentBlock.NewOCCBlockItem(1, 1)
	require.NoError(t, err)

	location := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}
	prog := &interpreter.Program{}
	state := &state.State{}

	txn.Set(location, ProgramEntry{
		Location: location,
		Program:  prog,
		State:    state,
	})

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
	require.Equal(t, location, parentEntry.Entry.Location)
	require.Same(t, prog, parentEntry.Entry.Program)
	require.Same(t, state, parentEntry.Entry.State)

	// Verify child is correctly initialized

	childBlock := parentBlock.NewChildOCCBlock()

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
	require.Equal(t, location, childEntry.Entry.Location)
	require.Same(t, prog, childEntry.Entry.Program)
	require.Same(t, state, childEntry.Entry.State)

	require.NotSame(t, parentEntry, childEntry)
}

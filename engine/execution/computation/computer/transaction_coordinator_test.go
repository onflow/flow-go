package computer

import (
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type testCoordinatorVM struct{}

func (testCoordinatorVM) NewExecutor(
	ctx fvm.Context,
	proc fvm.Procedure,
	txnState storage.TransactionPreparer,
) fvm.ProcedureExecutor {
	return testCoordinatorExecutor{
		executionTime: proc.ExecutionTime(),
	}
}

func (testCoordinatorVM) Run(
	ctx fvm.Context,
	proc fvm.Procedure,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*snapshot.ExecutionSnapshot,
	fvm.ProcedureOutput,
	error,
) {
	panic("not implemented")
}

func (testCoordinatorVM) GetAccount(
	_ fvm.Context,
	_ flow.Address,
	_ snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	panic("not implemented")
}

type testCoordinatorExecutor struct {
	executionTime logical.Time
}

func (testCoordinatorExecutor) Cleanup() {}

func (testCoordinatorExecutor) Preprocess() error {
	return nil
}

func (testCoordinatorExecutor) Execute() error {
	return nil
}

func (executor testCoordinatorExecutor) Output() fvm.ProcedureOutput {
	return fvm.ProcedureOutput{
		ComputationUsed: uint64(executor.executionTime),
	}
}

type testCoordinator struct {
	*transactionCoordinator
	committed []uint64
}

func newTestCoordinator(t *testing.T) *testCoordinator {
	db := &testCoordinator{}
	db.transactionCoordinator = newTransactionCoordinator(
		testCoordinatorVM{},
		nil,
		nil,
		db)

	require.Equal(t, db.SnapshotTime(), logical.Time(0))

	// commit a transaction to increment the snapshot time
	setupTxn, err := db.newTransaction(0)
	require.NoError(t, err)

	err = setupTxn.Finalize()
	require.NoError(t, err)

	err = setupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, db.SnapshotTime(), logical.Time(1))

	return db

}

func (db *testCoordinator) AddTransactionResult(
	txn TransactionRequest,
	snapshot *snapshot.ExecutionSnapshot,
	output fvm.ProcedureOutput,
	timeSpent time.Duration,
	numConflictRetries int,
) {
	db.committed = append(db.committed, output.ComputationUsed)
}

func (db *testCoordinator) newTransaction(txnIndex uint32) (
	*transaction,
	error,
) {
	return db.NewTransaction(
		newTransactionRequest(
			collectionInfo{},
			fvm.NewContext(),
			zerolog.Nop(),
			txnIndex,
			&flow.TransactionBody{},
			false),
		0)
}

type testWaitValues struct {
	startTime    logical.Time
	startErr     error
	snapshotTime logical.Time
	abortErr     error
}

func (db *testCoordinator) setupWait(txn *transaction) chan testWaitValues {
	ret := make(chan testWaitValues, 1)
	go func() {
		startTime, startErr, snapshotTime, abortErr := db.waitForUpdatesNewerThan(
			txn.SnapshotTime())
		ret <- testWaitValues{
			startTime:    startTime,
			startErr:     startErr,
			snapshotTime: snapshotTime,
			abortErr:     abortErr,
		}
	}()

	// Sleep a bit to ensure goroutine is running before returning the channel.
	time.Sleep(10 * time.Millisecond)
	return ret
}

func TestTransactionCoordinatorBasicCommit(t *testing.T) {
	db := newTestCoordinator(t)

	txns := []*transaction{}
	for i := uint32(1); i < 6; i++ {
		txn, err := db.newTransaction(i)
		require.NoError(t, err)

		txns = append(txns, txn)
	}

	for i, txn := range txns {
		executionTime := logical.Time(1 + i)

		require.Equal(t, txn.SnapshotTime(), logical.Time(1))

		err := txn.Finalize()
		require.NoError(t, err)

		err = txn.Validate()
		require.NoError(t, err)

		require.Equal(t, txn.SnapshotTime(), executionTime)

		err = txn.Commit()
		require.NoError(t, err)

		require.Equal(t, db.SnapshotTime(), executionTime+1)
	}

	require.Equal(t, db.committed, []uint64{0, 1, 2, 3, 4, 5})
}

func TestTransactionCoordinatorBlockingWaitForCommit(t *testing.T) {
	db := newTestCoordinator(t)

	testTxn, err := db.newTransaction(6)
	require.NoError(t, err)

	require.Equal(t, db.SnapshotTime(), logical.Time(1))
	require.Equal(t, testTxn.SnapshotTime(), logical.Time(1))

	ret := db.setupWait(testTxn)

	setupTxn, err := db.newTransaction(1)
	require.NoError(t, err)

	err = setupTxn.Finalize()
	require.NoError(t, err)

	err = setupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, db.SnapshotTime(), logical.Time(2))

	select {
	case val := <-ret:
		require.Equal(
			t,
			val,
			testWaitValues{
				startTime:    1,
				startErr:     nil,
				snapshotTime: 2,
				abortErr:     nil,
			})
	case <-time.After(time.Second):
		require.Fail(t, "Failed to return result")
	}

	require.Equal(t, testTxn.SnapshotTime(), logical.Time(1))

	err = testTxn.Validate()
	require.NoError(t, err)

	require.Equal(t, testTxn.SnapshotTime(), logical.Time(2))

}

func TestTransactionCoordinatorNonblockingWaitForCommit(t *testing.T) {
	db := newTestCoordinator(t)

	testTxn, err := db.newTransaction(6)
	require.NoError(t, err)

	setupTxn, err := db.newTransaction(1)
	require.NoError(t, err)

	err = setupTxn.Finalize()
	require.NoError(t, err)

	err = setupTxn.Commit()
	require.NoError(t, err)

	require.Equal(t, db.SnapshotTime(), logical.Time(2))
	require.Equal(t, testTxn.SnapshotTime(), logical.Time(1))

	ret := db.setupWait(testTxn)

	select {
	case val := <-ret:
		require.Equal(
			t,
			val,
			testWaitValues{
				startTime:    2,
				startErr:     nil,
				snapshotTime: 2,
				abortErr:     nil,
			})
	case <-time.After(time.Second):
		require.Fail(t, "Failed to return result")
	}
}

func TestTransactionCoordinatorBasicAbort(t *testing.T) {
	db := newTestCoordinator(t)

	txn, err := db.newTransaction(1)
	require.NoError(t, err)

	abortErr := fmt.Errorf("abort")
	db.AbortAllOutstandingTransactions(abortErr)

	err = txn.Finalize()
	require.NoError(t, err)

	err = txn.Commit()
	require.Equal(t, err, abortErr)

	txn, err = db.newTransaction(2)
	require.Equal(t, err, abortErr)
	require.Nil(t, txn)
}

func TestTransactionCoordinatorBlockingWaitForAbort(t *testing.T) {
	db := newTestCoordinator(t)

	testTxn, err := db.newTransaction(6)
	require.NoError(t, err)

	// start waiting before aborting.
	require.Equal(t, testTxn.SnapshotTime(), logical.Time(1))
	ret := db.setupWait(testTxn)

	abortErr := fmt.Errorf("abort")
	db.AbortAllOutstandingTransactions(abortErr)

	select {
	case val := <-ret:
		require.Equal(
			t,
			val,
			testWaitValues{
				startTime:    1,
				startErr:     nil,
				snapshotTime: 1,
				abortErr:     abortErr,
			})
	case <-time.After(time.Second):
		require.Fail(t, "Failed to return result")
	}

	err = testTxn.Finalize()
	require.NoError(t, err)

	err = testTxn.Commit()
	require.Equal(t, err, abortErr)
}

func TestTransactionCoordinatorNonblockingWaitForAbort(t *testing.T) {
	db := newTestCoordinator(t)

	testTxn, err := db.newTransaction(6)
	require.NoError(t, err)

	// start aborting before waiting.
	abortErr := fmt.Errorf("abort")
	db.AbortAllOutstandingTransactions(abortErr)

	require.Equal(t, testTxn.SnapshotTime(), logical.Time(1))
	ret := db.setupWait(testTxn)

	select {
	case val := <-ret:
		require.Equal(
			t,
			val,
			testWaitValues{
				startTime:    1,
				startErr:     abortErr,
				snapshotTime: 1,
				abortErr:     abortErr,
			})
	case <-time.After(time.Second):
		require.Fail(t, "Failed to return result")
	}

	err = testTxn.Finalize()
	require.NoError(t, err)

	err = testTxn.Commit()
	require.Equal(t, err, abortErr)
}

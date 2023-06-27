package computer

import (
	"sync"
	"time"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
)

type TransactionWriteBehindLogger interface {
	AddTransactionResult(
		txn TransactionRequest,
		snapshot *snapshot.ExecutionSnapshot,
		output fvm.ProcedureOutput,
		timeSpent time.Duration,
		numTxnConflictRetries int,
	)
}

// transactionCoordinator provides synchronization functionality for driving
// transaction execution.
type transactionCoordinator struct {
	vm fvm.VM

	mutex *sync.Mutex
	cond  *sync.Cond

	snapshotTime logical.Time // guarded by mutex, cond broadcast on updates.
	abortErr     error        // guarded by mutex, cond broadcast on updates.

	// Note: database commit and result logging must occur within the same
	// critical section (guraded by mutex).
	database       *storage.BlockDatabase
	writeBehindLog TransactionWriteBehindLogger
}

type transaction struct {
	request            TransactionRequest
	numConflictRetries int

	coordinator *transactionCoordinator

	startedAt time.Time
	storage.Transaction
	fvm.ProcedureExecutor
}

func newTransactionCoordinator(
	vm fvm.VM,
	storageSnapshot snapshot.StorageSnapshot,
	cachedDerivedBlockData *derived.DerivedBlockData,
	writeBehindLog TransactionWriteBehindLogger,
) *transactionCoordinator {
	mutex := &sync.Mutex{}
	cond := sync.NewCond(mutex)

	database := storage.NewBlockDatabase(
		storageSnapshot,
		0,
		cachedDerivedBlockData)

	return &transactionCoordinator{
		vm:             vm,
		mutex:          mutex,
		cond:           cond,
		snapshotTime:   0,
		abortErr:       nil,
		database:       database,
		writeBehindLog: writeBehindLog,
	}
}

func (coordinator *transactionCoordinator) SnapshotTime() logical.Time {
	coordinator.mutex.Lock()
	defer coordinator.mutex.Unlock()

	return coordinator.snapshotTime
}

func (coordinator *transactionCoordinator) Error() error {
	coordinator.mutex.Lock()
	defer coordinator.mutex.Unlock()

	return coordinator.abortErr
}

func (coordinator *transactionCoordinator) AbortAllOutstandingTransactions(
	err error,
) {
	coordinator.mutex.Lock()
	defer coordinator.mutex.Unlock()

	if coordinator.abortErr != nil { // Transactions are already aborting.
		return
	}

	coordinator.abortErr = err
	coordinator.cond.Broadcast()
}

func (coordinator *transactionCoordinator) NewTransaction(
	request TransactionRequest,
	attempt int,
) (
	*transaction,
	error,
) {
	err := coordinator.Error()
	if err != nil {
		return nil, err
	}

	txn, err := coordinator.database.NewTransaction(
		request.ExecutionTime(),
		fvm.ProcedureStateParameters(request.ctx, request))
	if err != nil {
		return nil, err
	}

	return &transaction{
		request:            request,
		coordinator:        coordinator,
		numConflictRetries: attempt,
		startedAt:          time.Now(),
		Transaction:        txn,
		ProcedureExecutor: coordinator.vm.NewExecutor(
			request.ctx,
			request.TransactionProcedure,
			txn),
	}, nil
}

func (coordinator *transactionCoordinator) commit(txn *transaction) error {
	coordinator.mutex.Lock()
	defer coordinator.mutex.Unlock()

	if coordinator.abortErr != nil {
		return coordinator.abortErr
	}

	executionSnapshot, err := txn.Transaction.Commit()
	if err != nil {
		return err
	}

	coordinator.writeBehindLog.AddTransactionResult(
		txn.request,
		executionSnapshot,
		txn.Output(),
		time.Since(txn.startedAt),
		txn.numConflictRetries)

	// Commit advances the database's snapshot.
	coordinator.snapshotTime += 1
	coordinator.cond.Broadcast()

	return nil
}

func (txn *transaction) Commit() error {
	return txn.coordinator.commit(txn)
}

func (coordinator *transactionCoordinator) waitForUpdatesNewerThan(
	snapshotTime logical.Time,
) (
	logical.Time,
	error,
	logical.Time,
	error,
) {
	coordinator.mutex.Lock()
	defer coordinator.mutex.Unlock()

	startTime := coordinator.snapshotTime
	startErr := coordinator.abortErr
	for coordinator.snapshotTime <= snapshotTime && coordinator.abortErr == nil {
		coordinator.cond.Wait()
	}

	return startTime, startErr, coordinator.snapshotTime, coordinator.abortErr
}

func (txn *transaction) WaitForUpdates() error {
	// Note: the frist three returned values are only used by tests to ensure
	// the function correctly waited.
	_, _, _, err := txn.coordinator.waitForUpdatesNewerThan(txn.SnapshotTime())
	return err
}

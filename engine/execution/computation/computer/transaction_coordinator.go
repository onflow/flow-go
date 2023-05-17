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

type transactionWriteBehindLogger interface {
	AddTransactionResult(
		txn transactionRequest,
		snapshot *snapshot.ExecutionSnapshot,
		output fvm.ProcedureOutput,
		timeSpent time.Duration,
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
	database *storage.BlockDatabase
	log      transactionWriteBehindLogger
}

type transaction struct {
	request transactionRequest

	coordinator *transactionCoordinator

	startedAt time.Time
	storage.Transaction
	fvm.ProcedureExecutor
}

func newTransactionCoordinator(
	vm fvm.VM,
	storageSnapshot snapshot.StorageSnapshot,
	cachedDerivedBlockData *derived.DerivedBlockData,
	log transactionWriteBehindLogger,
) *transactionCoordinator {
	mutex := &sync.Mutex{}
	cond := sync.NewCond(mutex)

	database := storage.NewBlockDatabase(
		storageSnapshot,
		0,
		cachedDerivedBlockData)

	return &transactionCoordinator{
		vm:           vm,
		mutex:        mutex,
		cond:         cond,
		snapshotTime: 0,
		abortErr:     nil,
		database:     database,
		log:          log,
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
	request transactionRequest,
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
		request:     request,
		coordinator: coordinator,
		startedAt:   time.Now(),
		Transaction: txn,
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

	coordinator.log.AddTransactionResult(
		txn.request,
		executionSnapshot,
		txn.Output(),
		time.Since(txn.startedAt))

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
	logical.Time,
	error,
) {
	coordinator.mutex.Lock()
	defer coordinator.mutex.Unlock()

	startTime := coordinator.snapshotTime
	for {
		if coordinator.snapshotTime > snapshotTime {
			return startTime, coordinator.snapshotTime, coordinator.abortErr
		}

		if coordinator.abortErr != nil {
			return startTime, coordinator.snapshotTime, coordinator.abortErr
		}

		coordinator.cond.Wait()
	}
}

func (txn *transaction) WaitForUpdates() error {
	_, _, err := txn.coordinator.waitForUpdatesNewerThan(txn.SnapshotTime())
	return err
}

package primary

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/fvm/storage/errors"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
)

const (
	conflictErrorTemplate = "invalid transaction: committed txn %d conflicts " +
		"with executing txn %d with snapshot at %d (Conflicting register: %v)"
)

// BlockData is a rudimentary in-memory MVCC database for storing (RegisterID,
// RegisterValue) pairs for a particular block.  The database enforces
// atomicity, consistency, and isolation, but not durability (The transactions
// are made durable by the block computer using aggregated execution snapshots).
type BlockData struct {
	mutex sync.RWMutex

	latestSnapshot timestampedSnapshotTree // Guarded by mutex
}

type TransactionData struct {
	block *BlockData

	executionTime             logical.Time
	isSnapshotReadTransaction bool

	snapshot *rebaseableTimestampedSnapshotTree

	state.NestedTransactionPreparer

	finalizedExecutionSnapshot *snapshot.ExecutionSnapshot
}

// Note: storageSnapshot must be thread safe.
func NewBlockData(
	storageSnapshot snapshot.StorageSnapshot,
	snapshotTime logical.Time,
) *BlockData {
	return &BlockData{
		latestSnapshot: newTimestampedSnapshotTree(
			storageSnapshot,
			logical.Time(snapshotTime)),
	}
}

func (block *BlockData) LatestSnapshot() timestampedSnapshotTree {
	block.mutex.RLock()
	defer block.mutex.RUnlock()

	return block.latestSnapshot
}

func (block *BlockData) newTransactionData(
	isSnapshotReadTransaction bool,
	executionTime logical.Time,
	parameters state.StateParameters,
) *TransactionData {
	snapshot := newRebaseableTimestampedSnapshotTree(block.LatestSnapshot())
	return &TransactionData{
		block:                     block,
		executionTime:             executionTime,
		snapshot:                  snapshot,
		isSnapshotReadTransaction: isSnapshotReadTransaction,
		NestedTransactionPreparer: state.NewTransactionState(
			snapshot,
			parameters),
	}
}

func (block *BlockData) NewTransactionData(
	executionTime logical.Time,
	parameters state.StateParameters,
) (
	*TransactionData,
	error,
) {
	if executionTime < 0 ||
		executionTime > logical.LargestNormalTransactionExecutionTime {

		return nil, fmt.Errorf(
			"invalid tranaction: execution time out of bound")
	}

	txn := block.newTransactionData(
		false,
		executionTime,
		parameters)

	if txn.SnapshotTime() > executionTime {
		return nil, fmt.Errorf(
			"invalid transaction: snapshot > execution: %v > %v",
			txn.SnapshotTime(),
			executionTime)
	}

	return txn, nil
}

func (block *BlockData) NewSnapshotReadTransactionData(
	parameters state.StateParameters,
) *TransactionData {
	return block.newTransactionData(
		true,
		logical.EndOfBlockExecutionTime,
		parameters)
}

func (txn *TransactionData) SnapshotTime() logical.Time {
	return txn.snapshot.SnapshotTime()
}

func (txn *TransactionData) validate(
	latestSnapshot timestampedSnapshotTree,
) error {
	validatedSnapshotTime := txn.SnapshotTime()

	if latestSnapshot.SnapshotTime() <= validatedSnapshotTime {
		// transaction's snapshot is up-to-date.
		return nil
	}

	var readSet map[flow.RegisterID]struct{}
	if txn.finalizedExecutionSnapshot != nil {
		readSet = txn.finalizedExecutionSnapshot.ReadSet
	} else {
		readSet = txn.InterimReadSet()
	}

	updates, err := latestSnapshot.UpdatesSince(validatedSnapshotTime)
	if err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}

	for i, writeSet := range updates {
		hasConflict, registerId := intersect(writeSet, readSet)
		if hasConflict {
			return errors.NewRetryableConflictError(
				conflictErrorTemplate,
				validatedSnapshotTime+logical.Time(i),
				txn.executionTime,
				validatedSnapshotTime,
				registerId)
		}
	}

	txn.snapshot.Rebase(latestSnapshot)
	return nil
}

func (txn *TransactionData) Validate() error {
	return txn.validate(txn.block.LatestSnapshot())
}

func (txn *TransactionData) Finalize() error {
	executionSnapshot, err := txn.FinalizeMainTransaction()
	if err != nil {
		return err
	}

	// NOTE: Since cadence does not support the notion of read only execution,
	// snapshot read transaction execution can inadvertently produce a non-empty
	// write set.  We'll just drop these updates.
	if txn.isSnapshotReadTransaction {
		executionSnapshot.WriteSet = nil
	}

	txn.finalizedExecutionSnapshot = executionSnapshot
	return nil
}

func (block *BlockData) commit(txn *TransactionData) error {
	if txn.finalizedExecutionSnapshot == nil {
		return fmt.Errorf("invalid transaction: transaction not finalized.")
	}

	block.mutex.Lock()
	defer block.mutex.Unlock()

	err := txn.validate(block.latestSnapshot)
	if err != nil {
		return err
	}

	// Don't perform actual commit for snapshot read transaction since they
	// do not advance logical time.
	if txn.isSnapshotReadTransaction {
		return nil
	}

	latestSnapshotTime := block.latestSnapshot.SnapshotTime()

	if latestSnapshotTime < txn.executionTime {
		// i.e., transactions are committed out-of-order.
		return fmt.Errorf(
			"invalid transaction: missing commit range [%v, %v)",
			latestSnapshotTime,
			txn.executionTime)
	}

	if block.latestSnapshot.SnapshotTime() > txn.executionTime {
		// i.e., re-commiting an already committed transaction.
		return fmt.Errorf(
			"invalid transaction: non-increasing time (%v >= %v)",
			latestSnapshotTime-1,
			txn.executionTime)
	}

	block.latestSnapshot = block.latestSnapshot.Append(
		txn.finalizedExecutionSnapshot)

	return nil
}

func (txn *TransactionData) Commit() (
	*snapshot.ExecutionSnapshot,
	error,
) {
	err := txn.block.commit(txn)
	if err != nil {
		return nil, err
	}

	return txn.finalizedExecutionSnapshot, nil
}

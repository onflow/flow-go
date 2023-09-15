package derived

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/onflow/flow-go/fvm/storage/errors"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
)

// ValueComputer is used by DerivedDataTable's GetOrCompute to compute the
// derived value when the value is not in DerivedDataTable (i.e., "cache miss").
type ValueComputer[TKey any, TVal any] interface {
	Compute(txnState state.NestedTransactionPreparer, key TKey) (TVal, error)
}

type invalidatableEntry[TVal any] struct {
	Value             TVal                        // immutable after initialization.
	ExecutionSnapshot *snapshot.ExecutionSnapshot // immutable after initialization.

	isInvalid bool // Guarded by DerivedDataTable' lock.
}

// DerivedDataTable is a rudimentary fork-aware OCC database table for
// "caching" homogeneous (TKey, TVal) pairs for a particular block.
//
// The database table enforces atomicity and isolation, but not consistency and
// durability.  Consistency depends on the user correctly implementing the
// table's invalidator.  Durability is not needed since the values are derived
// from ledger and can be computed on the fly (This assumes that recomputation
// is idempotent).
//
// Furthermore, because data are derived, transaction validation looks
// a bit unusual when compared with a textbook OCC implementation.  In
// particular, the transaction's invalidator represents "real" writes to the
// canonical source, whereas the transaction's readSet/writeSet entries
// represent "real" reads from the canonical source.
//
// Multiple tables are grouped together via Validate/Commit 2 phase commit to
// form the complete derived data database.
type DerivedDataTable[TKey comparable, TVal any] struct {
	lock  sync.RWMutex
	items map[TKey]*invalidatableEntry[TVal]

	latestCommitExecutionTime logical.Time

	invalidators chainedTableInvalidators[TKey, TVal] // Guarded by lock.
}

type TableTransaction[TKey comparable, TVal any] struct {
	table *DerivedDataTable[TKey, TVal]

	// The start time when the snapshot first becomes readable (i.e., the
	// "snapshotTime - 1"'s transaction committed the snapshot view).
	snapshotTime logical.Time

	// The transaction (or script)'s execution start time (aka TxIndex).
	executionTime logical.Time

	// toValidateTime is used to amortize cost of repeated Validate calls.
	// Each Validate call will only validate the time range
	// [toValidateTime, executionTime), and will advance toValidateTime to
	// latestCommitExecutionTime + 1 if Validate succeeded.
	//
	// Note that since newly derived values are computed based on snapshotTime's
	// view, each time a newly derived value is added to the transaction,
	// toValidateTime is reset back to snapshotTime.
	toValidateTime logical.Time

	readSet  map[TKey]*invalidatableEntry[TVal]
	writeSet map[TKey]*invalidatableEntry[TVal]

	// When isSnapshotReadTransaction is true, invalidators must be empty.
	isSnapshotReadTransaction bool
	invalidators              chainedTableInvalidators[TKey, TVal]
}

func NewEmptyTable[
	TKey comparable,
	TVal any,
](
	initialSnapshotTime logical.Time,
) *DerivedDataTable[TKey, TVal] {
	return &DerivedDataTable[TKey, TVal]{
		items:                     map[TKey]*invalidatableEntry[TVal]{},
		latestCommitExecutionTime: initialSnapshotTime - 1,
		invalidators:              nil,
	}
}

func (table *DerivedDataTable[TKey, TVal]) NewChildTable() *DerivedDataTable[TKey, TVal] {
	table.lock.RLock()
	defer table.lock.RUnlock()

	items := make(
		map[TKey]*invalidatableEntry[TVal],
		len(table.items))

	for key, entry := range table.items {
		// Note: We need to deep copy the invalidatableEntry here since the
		// entry may be valid in the parent table, but invalid in the child
		// table.
		items[key] = &invalidatableEntry[TVal]{
			Value:             entry.Value,
			ExecutionSnapshot: entry.ExecutionSnapshot,
			isInvalid:         false,
		}
	}

	return &DerivedDataTable[TKey, TVal]{
		items:                     items,
		latestCommitExecutionTime: logical.ParentBlockTime,
		invalidators:              nil,
	}
}

func (table *DerivedDataTable[TKey, TVal]) NextTxIndexForTestingOnly() uint32 {
	return uint32(table.LatestCommitExecutionTimeForTestingOnly()) + 1
}

func (table *DerivedDataTable[TKey, TVal]) LatestCommitExecutionTimeForTestingOnly() logical.Time {
	table.lock.RLock()
	defer table.lock.RUnlock()

	return table.latestCommitExecutionTime
}

func (table *DerivedDataTable[TKey, TVal]) EntriesForTestingOnly() map[TKey]*invalidatableEntry[TVal] {
	table.lock.RLock()
	defer table.lock.RUnlock()

	entries := make(
		map[TKey]*invalidatableEntry[TVal],
		len(table.items))
	for key, entry := range table.items {
		entries[key] = entry
	}

	return entries
}

func (table *DerivedDataTable[TKey, TVal]) InvalidatorsForTestingOnly() chainedTableInvalidators[TKey, TVal] {
	table.lock.RLock()
	defer table.lock.RUnlock()

	return table.invalidators
}

func (table *DerivedDataTable[TKey, TVal]) GetForTestingOnly(
	key TKey,
) *invalidatableEntry[TVal] {
	return table.get(key)
}

func (table *DerivedDataTable[TKey, TVal]) get(
	key TKey,
) *invalidatableEntry[TVal] {
	table.lock.RLock()
	defer table.lock.RUnlock()

	return table.items[key]
}

func (table *DerivedDataTable[TKey, TVal]) unsafeValidate(
	txn *TableTransaction[TKey, TVal],
) error {
	if txn.isSnapshotReadTransaction &&
		txn.invalidators.ShouldInvalidateEntries() {

		return fmt.Errorf(
			"invalid TableTransaction: snapshot read can't invalidate")
	}

	if table.latestCommitExecutionTime >= txn.executionTime {
		return fmt.Errorf(
			"invalid TableTransaction: non-increasing time (%v >= %v)",
			table.latestCommitExecutionTime,
			txn.executionTime)
	}

	for _, entry := range txn.readSet {
		if entry.isInvalid {
			if txn.snapshotTime == txn.executionTime {
				// This should never happen since the transaction is
				// sequentially executed.
				return fmt.Errorf(
					"invalid TableTransaction: unrecoverable outdated read set")
			}

			return errors.NewRetryableConflictError(
				"invalid TableTransaction: outdated read set")
		}
	}

	applicable := table.invalidators.ApplicableInvalidators(
		txn.toValidateTime)
	shouldInvalidateEntries := applicable.ShouldInvalidateEntries()

	for key, entry := range txn.writeSet {
		current, ok := table.items[key]
		if ok && current != entry {
			// The derived data table must always return the same item for a given key,
			// otherwise the cadence runtime will have issues comparing resolved cadence types.
			//
			// for example:
			// two transactions are run concurrently, first loads (cadence contracts)
			// A and B where B depends on A. The second transaction also loads A and C,
			// where C depends on A. The first transaction commits first.
			// The A from the second transaction is equivalent to the A from
			// the first transaction but it is not the same object.
			//
			// Overwriting A with the A from the second transaction will cause program B
			// to break because it will not know the types from A returned from
			// the cache in the future.
			// Not overwriting A will cause program C to break because it will not know
			// the types from A returned from the cache in the future.
			//
			// The solution is to treat this as a conflict and retry the transaction.
			// When the transaction is retried, the A from the first transaction will
			// be used to load C in the second transaction.

			return errors.NewRetryableConflictError(
				"invalid TableTransaction: write conflict")
		}

		if !shouldInvalidateEntries ||
			!applicable.ShouldInvalidateEntry(
				key,
				entry.Value,
				entry.ExecutionSnapshot,
			) {
			continue
		}

		if txn.snapshotTime == txn.executionTime {
			// This should never happen since the transaction is
			// sequentially executed.
			return fmt.Errorf(
				"invalid TableTransaction: unrecoverable outdated " +
					"write set")
		}

		return errors.NewRetryableConflictError(
			"invalid TableTransaction: outdated write set")

	}

	txn.toValidateTime = table.latestCommitExecutionTime + 1

	return nil
}

func (table *DerivedDataTable[TKey, TVal]) validate(
	txn *TableTransaction[TKey, TVal],
) error {
	table.lock.RLock()
	defer table.lock.RUnlock()

	return table.unsafeValidate(txn)
}

func (table *DerivedDataTable[TKey, TVal]) commit(
	txn *TableTransaction[TKey, TVal],
) error {
	table.lock.Lock()
	defer table.lock.Unlock()

	if !txn.isSnapshotReadTransaction &&
		table.latestCommitExecutionTime+1 < txn.snapshotTime {

		return fmt.Errorf(
			"invalid TableTransaction: missing commit range [%v, %v)",
			table.latestCommitExecutionTime+1,
			txn.snapshotTime)
	}

	// NOTE: Instead of throwing out all the write entries, we can commit
	// the valid write entries then return error.
	err := table.unsafeValidate(txn)
	if err != nil {
		return err
	}

	// Don't perform actual commit for snapshot read transaction.  This is
	// safe since all values are derived from the primary source.
	if txn.isSnapshotReadTransaction {
		return nil
	}

	for key, entry := range txn.writeSet {
		_, ok := table.items[key]
		if ok {
			// A previous transaction already committed an equivalent
			// TableTransaction entry.  Since both TableTransaction entry are
			// valid, just reuse the existing one for future transactions.
			continue
		}

		table.items[key] = entry
	}

	if txn.invalidators.ShouldInvalidateEntries() {
		for key, entry := range table.items {
			if txn.invalidators.ShouldInvalidateEntry(
				key,
				entry.Value,
				entry.ExecutionSnapshot) {

				entry.isInvalid = true
				delete(table.items, key)
			}
		}

		table.invalidators = append(
			table.invalidators,
			txn.invalidators...)
	}

	table.latestCommitExecutionTime = txn.executionTime
	return nil
}

func (table *DerivedDataTable[TKey, TVal]) newTableTransaction(
	snapshotTime logical.Time,
	executionTime logical.Time,
	isSnapshotReadTransaction bool,
) *TableTransaction[TKey, TVal] {
	return &TableTransaction[TKey, TVal]{
		table:                     table,
		snapshotTime:              snapshotTime,
		executionTime:             executionTime,
		toValidateTime:            snapshotTime,
		readSet:                   map[TKey]*invalidatableEntry[TVal]{},
		writeSet:                  map[TKey]*invalidatableEntry[TVal]{},
		isSnapshotReadTransaction: isSnapshotReadTransaction,
	}
}

func (table *DerivedDataTable[TKey, TVal]) NewSnapshotReadTableTransaction() *TableTransaction[TKey, TVal] {
	return table.newTableTransaction(
		logical.EndOfBlockExecutionTime,
		logical.EndOfBlockExecutionTime,
		true)
}

func (table *DerivedDataTable[TKey, TVal]) NewTableTransaction(
	snapshotTime logical.Time,
	executionTime logical.Time,
) (
	*TableTransaction[TKey, TVal],
	error,
) {
	if executionTime < 0 ||
		executionTime > logical.LargestNormalTransactionExecutionTime {

		return nil, fmt.Errorf(
			"invalid TableTransactions: execution time out of bound: %v",
			executionTime)
	}

	if snapshotTime > executionTime {
		return nil, fmt.Errorf(
			"invalid TableTransactions: snapshot > execution: %v > %v",
			snapshotTime,
			executionTime)
	}

	return table.newTableTransaction(
		snapshotTime,
		executionTime,
		false), nil
}

// Note: use GetOrCompute instead of Get/Set whenever possible.
func (txn *TableTransaction[TKey, TVal]) get(key TKey) (
	TVal,
	*snapshot.ExecutionSnapshot,
	bool,
) {

	writeEntry, ok := txn.writeSet[key]
	if ok {
		return writeEntry.Value, writeEntry.ExecutionSnapshot, true
	}

	readEntry := txn.readSet[key]
	if readEntry != nil {
		return readEntry.Value, readEntry.ExecutionSnapshot, true
	}

	readEntry = txn.table.get(key)
	if readEntry != nil {
		txn.readSet[key] = readEntry
		return readEntry.Value, readEntry.ExecutionSnapshot, true
	}

	var defaultValue TVal
	return defaultValue, nil, false
}

func (txn *TableTransaction[TKey, TVal]) GetForTestingOnly(key TKey) (
	TVal,
	*snapshot.ExecutionSnapshot,
	bool,
) {
	return txn.get(key)
}

func (txn *TableTransaction[TKey, TVal]) set(
	key TKey,
	value TVal,
	snapshot *snapshot.ExecutionSnapshot,
) {
	txn.writeSet[key] = &invalidatableEntry[TVal]{
		Value:             value,
		ExecutionSnapshot: snapshot,
		isInvalid:         false,
	}

	// Since value is derived from snapshot's view.  We need to reset the
	// toValidateTime back to snapshot time to re-validate the entry.
	txn.toValidateTime = txn.snapshotTime
}

func (txn *TableTransaction[TKey, TVal]) SetForTestingOnly(
	key TKey,
	value TVal,
	snapshot *snapshot.ExecutionSnapshot,
) {
	txn.set(key, value, snapshot)
}

// GetOrCompute returns the key's value.  If a pre-computed value is available,
// then the pre-computed value is returned and the cached state is replayed on
// txnState.  Otherwise, the value is computed using valFunc; both the value
// and the states used to compute the value are captured.
//
// Note: valFunc must be an idempotent function and it must not modify
// txnState's values.
func (txn *TableTransaction[TKey, TVal]) GetOrCompute(
	txnState state.NestedTransactionPreparer,
	key TKey,
	computer ValueComputer[TKey, TVal],
) (
	TVal,
	error,
) {
	var defaultVal TVal

	val, state, ok := txn.get(key)
	if ok {
		err := txnState.AttachAndCommitNestedTransaction(state)
		if err != nil {
			return defaultVal, fmt.Errorf(
				"failed to replay cached state: %w",
				err)
		}

		return val, nil
	}

	nestedTxId, err := txnState.BeginNestedTransaction()
	if err != nil {
		return defaultVal, fmt.Errorf("failed to start nested txn: %w", err)
	}

	val, err = computer.Compute(txnState, key)

	// Commit the nested transaction, even if the computation fails.
	committedState, commitErr := txnState.CommitNestedTransaction(nestedTxId)
	if commitErr != nil {
		err = multierror.Append(err,
			fmt.Errorf("failed to commit nested txn: %w", commitErr),
		).ErrorOrNil()
	}

	if err != nil {
		return defaultVal, fmt.Errorf("failed to derive value: %w", err)
	}

	txn.set(key, val, committedState)

	return val, nil
}

func (txn *TableTransaction[TKey, TVal]) AddInvalidator(
	invalidator TableInvalidator[TKey, TVal],
) {
	if invalidator == nil || !invalidator.ShouldInvalidateEntries() {
		return
	}

	txn.invalidators = append(
		txn.invalidators,
		tableInvalidatorAtTime[TKey, TVal]{
			TableInvalidator: invalidator,
			executionTime:    txn.executionTime,
		})
}

func (txn *TableTransaction[TKey, TVal]) Validate() error {
	return txn.table.validate(txn)
}

func (txn *TableTransaction[TKey, TVal]) Commit() error {
	return txn.table.commit(txn)
}

func (txn *TableTransaction[TKey, TVal]) ToValidateTimeForTestingOnly() logical.Time {
	return txn.toValidateTime
}

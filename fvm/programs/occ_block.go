package programs

import (
	"fmt"
	"sync"
)

type invalidatableEntry[TVal any] struct {
	Entry TVal // Immutable after initialization.

	isInvalid bool // Guarded by blockPrograms' lock.
}

// BlockPrograms is a simple fork-aware OCC database for "caching" programs
// for a particular block.
//
// Since programs are derived from external source, the database need not be
// durable and can be recreated on the fly.
//
// Furthermore, because programs are derived data, transaction validation looks
// a bit unusual when compared with a textbook OCC implementation.  In
// particular, the transaction's invalidator represents "real" writes to the
// canonical source, whereas the transaction's readSet/writeSet entries
// represent "real" reads from the canonical source.
type OCCBlock[TKey comparable, TVal any] struct {
	lock  sync.RWMutex
	items map[TKey]*invalidatableEntry[TVal]

	latestCommitExecutionTime LogicalTime

	invalidators chainedOCCInvalidators[TVal] // Guarded by lock.
}

type OCCBlockItem[TKey comparable, TVal any] struct {
	block *OCCBlock[TKey, TVal]

	// The start time when the snapshot first becomes readable (i.e., the
	// "snapshotTime - 1"'s transaction committed the snapshot view)
	snapshotTime LogicalTime

	// The transaction (or script)'s execution start time (aka TxIndex).
	executionTime LogicalTime

	readSet  map[TKey]*invalidatableEntry[TVal]
	writeSet map[TKey]TVal

	/*
		// NOTE: non-address programs are not reusable across transactions, hence
		// they are kept out of the writeSet and the BlockPrograms database.
		nonAddressSet map[common.Location]ProgramEntry
	*/

	// When isSnapshotReadTransaction is true, invalidators must be empty.
	isSnapshotReadTransaction bool
	invalidators              chainedOCCInvalidators[TVal]
}

func newEmptyOCCBlock[TKey comparable, TVal any](latestCommit LogicalTime) *OCCBlock[TKey, TVal] {
	return &OCCBlock[TKey, TVal]{
		items:                     map[TKey]*invalidatableEntry[TVal]{},
		latestCommitExecutionTime: latestCommit,
		invalidators:              nil,
	}
}

func NewEmptyOCCBlock[TKey comparable, TVal any]() *OCCBlock[TKey, TVal] {
	return newEmptyOCCBlock[TKey, TVal](ParentBlockTime)
}

// This variant is needed by the chunk verifier, which does not start at the
// beginning of the block.
func NewEmptyOCCBlockWithOffset[TKey comparable, TVal any](offset uint32) *OCCBlock[TKey, TVal] {
	return newEmptyOCCBlock[TKey, TVal](LogicalTime(offset) - 1)
}

func (block *OCCBlock[TKey, TVal]) NewChildOCCBlock() *OCCBlock[TKey, TVal] {
	block.lock.RLock()
	defer block.lock.RUnlock()

	items := make(
		map[TKey]*invalidatableEntry[TVal],
		len(block.items))

	for key, entry := range block.items {
		items[key] = &invalidatableEntry[TVal]{
			Entry:     entry.Entry,
			isInvalid: false,
		}
	}

	return &OCCBlock[TKey, TVal]{
		items:                     items,
		latestCommitExecutionTime: ParentBlockTime,
		invalidators:              nil,
	}
}

func (block *OCCBlock[TKey, TVal]) NextTxIndexForTestingOnly() uint32 {
	return uint32(block.LatestCommitExecutionTimeForTestingOnly()) + 1
}

func (block *OCCBlock[TKey, TVal]) LatestCommitExecutionTimeForTestingOnly() LogicalTime {
	block.lock.RLock()
	defer block.lock.RUnlock()

	return block.latestCommitExecutionTime
}

func (block *OCCBlock[TKey, TVal]) EntriesForTestingOnly() map[TKey]*invalidatableEntry[TVal] {
	block.lock.RLock()
	defer block.lock.RUnlock()

	entries := make(
		map[TKey]*invalidatableEntry[TVal],
		len(block.items))
	for key, entry := range block.items {
		entries[key] = entry
	}

	return entries
}

func (block *OCCBlock[TKey, TVal]) InvalidatorsForTestingOnly() chainedOCCInvalidators[TVal] {
	block.lock.RLock()
	defer block.lock.RUnlock()

	return block.invalidators
}

func (block *OCCBlock[TKey, TVal]) GetForTestingOnly(
	key TKey,
) *TVal {
	entry := block.get(key)
	if entry != nil {
		return &entry.Entry
	}
	return nil
}

func (block *OCCBlock[TKey, TVal]) get(
	key TKey,
) *invalidatableEntry[TVal] {
	block.lock.RLock()
	defer block.lock.RUnlock()

	return block.items[key]
}

func (block *OCCBlock[TKey, TVal]) unsafeValidate(
	item *OCCBlockItem[TKey, TVal],
) RetryableError {
	if item.isSnapshotReadTransaction &&
		item.invalidators.ShouldInvalidateItems() {

		return newNotRetryableError(
			"invalid TransactionProgram: snapshot read can't invalidate")
	}

	if block.latestCommitExecutionTime >= item.executionTime {
		return newNotRetryableError(
			"invalid TransactionProgram: non-increasing time (%v >= %v)",
			block.latestCommitExecutionTime,
			item.executionTime)
	}

	if block.latestCommitExecutionTime+1 < item.snapshotTime &&
		(!item.isSnapshotReadTransaction ||
			item.snapshotTime != EndOfBlockExecutionTime) {

		return newNotRetryableError(
			"invalid TransactionProgram: missing commit range [%v, %v)",
			block.latestCommitExecutionTime+1,
			item.snapshotTime)
	}

	for _, entry := range item.readSet {
		if entry.isInvalid {
			return newRetryableError(
				"invalid TransactionPrograms. outdated read set")
		}
	}

	applicable := block.invalidators.ApplicableInvalidators(
		item.snapshotTime)
	if applicable.ShouldInvalidateItems() {
		for _, entry := range item.writeSet {
			if applicable.ShouldInvalidateEntry(entry) {
				return newRetryableError(
					"invalid TransactionPrograms. outdated write set")
			}
		}
	}

	return nil
}

func (block *OCCBlock[TKey, TVal]) validate(
	item *OCCBlockItem[TKey, TVal],
) RetryableError {
	block.lock.RLock()
	defer block.lock.RUnlock()

	return block.unsafeValidate(item)
}

func (block *OCCBlock[TKey, TVal]) commit(
	item *OCCBlockItem[TKey, TVal],
) RetryableError {
	block.lock.Lock()
	defer block.lock.Unlock()

	// NOTE: Instead of throwing out all the write entries, we can commit
	// the valid write entries then return error.
	err := block.unsafeValidate(item)
	if err != nil {
		return err
	}

	for key, entry := range item.writeSet {
		_, ok := block.items[key]
		if ok {
			// A previous transaction already committed an equivalent program
			// entry.  Since both program entry are valid, just reuse the
			// existing one for future transactions.
			continue
		}

		block.items[key] = &invalidatableEntry[TVal]{
			Entry:     entry,
			isInvalid: false,
		}
	}

	if item.invalidators.ShouldInvalidateItems() {
		for key, entry := range block.items {
			if item.invalidators.ShouldInvalidateEntry(
				entry.Entry) {

				entry.isInvalid = true
				delete(block.items, key)
			}
		}

		block.invalidators = append(
			block.invalidators,
			item.invalidators...)
	}

	// NOTE: We cannot advance commit time when we encounter a snapshot read
	// (aka script) transaction since these transactions don't generate new
	// snapshots.  It is safe to commit the entries since snapshot read
	// transactions never invalidate entries.
	if !item.isSnapshotReadTransaction {
		block.latestCommitExecutionTime = item.executionTime
	}
	return nil
}

func (block *OCCBlock[TKey, TVal]) newOCCBlockItem(
	upperBoundExecutionTime LogicalTime,
	snapshotTime LogicalTime,
	executionTime LogicalTime,
	isSnapshotReadTransaction bool,
) (
	*OCCBlockItem[TKey, TVal],
	error,
) {
	if executionTime < 0 || executionTime > upperBoundExecutionTime {
		return nil, fmt.Errorf(
			"invalid TransactionPrograms: execution time out of bound: %v",
			executionTime)
	}

	if snapshotTime > executionTime {
		return nil, fmt.Errorf(
			"invalid TransactionPrograms: snapshot > execution: %v > %v",
			snapshotTime,
			executionTime)
	}

	return &OCCBlockItem[TKey, TVal]{
		block:                     block,
		snapshotTime:              snapshotTime,
		executionTime:             executionTime,
		readSet:                   map[TKey]*invalidatableEntry[TVal]{},
		writeSet:                  map[TKey]TVal{},
		isSnapshotReadTransaction: isSnapshotReadTransaction,
	}, nil
}

func (block *OCCBlock[TKey, TVal]) NewSnapshotReadOCCBlockItem(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*OCCBlockItem[TKey, TVal],
	error,
) {
	return block.newOCCBlockItem(
		LargestSnapshotReadTransactionExecutionTime,
		snapshotTime,
		executionTime,
		true)
}

func (block *OCCBlock[TKey, TVal]) NewOCCBlockItem(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*OCCBlockItem[TKey, TVal],
	error,
) {
	return block.newOCCBlockItem(
		LargestNormalTransactionExecutionTime,
		snapshotTime,
		executionTime,
		false)
}

func (item *OCCBlockItem[TKey, TVal]) Get(
	key TKey,
) *TVal { // TODO: no pointer?
	/*
		_, ok := location.(common.AddressLocation)
		if !ok {
			nonAddrEntry, ok := transaction.nonAddressSet[location]
			return nonAddrEntry.Program, nonAddrEntry.State, ok
		}
	*/

	writeEntry, ok := item.writeSet[key]
	if ok {
		return &writeEntry
	}

	readEntry := item.readSet[key]
	if readEntry != nil {
		return &readEntry.Entry
	}

	readEntry = item.block.get(key)
	if readEntry != nil {
		item.readSet[key] = readEntry
		return &readEntry.Entry
	}

	return nil
}

func (item *OCCBlockItem[TKey, TVal]) Set(
	key TKey,
	val TVal,
) {
	/*
		addrLoc, ok := location.(common.AddressLocation)
		if !ok {
			transaction.nonAddressSet[location] = ProgramEntry{
				Program: program,
				State:   state,
			}
			return
		}
	*/
	item.writeSet[key] = val
}

func (item *OCCBlockItem[TKey, TVal]) AddInvalidator(
	invalidator OCCInvalidator[TVal],
) {
	if invalidator == nil || !invalidator.ShouldInvalidateItems() {
		return
	}

	item.invalidators = append(
		item.invalidators,
		occInvalidatorAtTime[TVal]{
			OCCInvalidator: invalidator,
			executionTime:  item.executionTime,
		})
}

func (item *OCCBlockItem[TKey, TVal]) Validate() RetryableError {
	return item.block.validate(item)
}

func (item *OCCBlockItem[TKey, TVal]) Commit() RetryableError {
	return item.block.commit(item)
}

package programs

import (
	"fmt"
	"sync"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
)

type invalidatableProgramEntry struct {
	ProgramEntry // Immutable after initialization.

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
type BlockPrograms struct {
	lock     sync.RWMutex
	programs map[common.Location]*invalidatableProgramEntry

	latestCommitExecutionTime LogicalTime

	invalidators chainedInvalidators // Guarded by lock.
}

// TransactionPrograms is the scratch space for a single transaction.
type TransactionPrograms struct {
	block *BlockPrograms

	// The start time when the snapshot first becomes readable (i.e., the
	// "snapshotTime - 1"'s transaction committed the snapshot view)
	snapshotTime LogicalTime

	// The transaction (or script)'s execution start time (aka TxIndex).
	executionTime LogicalTime

	readSet  map[common.Location]*invalidatableProgramEntry
	writeSet map[common.Location]ProgramEntry

	// NOTE: non-address programs are not reusable across transactions, hence
	// they are kept out of the writeSet and the BlockPrograms database.
	nonAddressSet map[common.Location]ProgramEntry

	// When isSnapshotReadTransaction is true, invalidators must be empty.
	isSnapshotReadTransaction bool
	invalidators              chainedInvalidators
}

func newEmptyBlockPrograms(latestCommit LogicalTime) *BlockPrograms {
	return &BlockPrograms{
		programs:                  map[common.Location]*invalidatableProgramEntry{},
		latestCommitExecutionTime: latestCommit,
		invalidators:              nil,
	}
}

func NewEmptyBlockPrograms() *BlockPrograms {
	return newEmptyBlockPrograms(ParentBlockTime)
}

// This variant is needed by the chunk verifier, which does not start at the
// beginning of the block.
func NewEmptyBlockProgramsWithTransactionOffset(offset uint32) *BlockPrograms {
	return newEmptyBlockPrograms(LogicalTime(offset) - 1)
}

func (block *BlockPrograms) NewChildBlockPrograms() *BlockPrograms {
	block.lock.RLock()
	defer block.lock.RUnlock()

	programs := make(
		map[common.Location]*invalidatableProgramEntry,
		len(block.programs))

	for locId, entry := range block.programs {
		programs[locId] = &invalidatableProgramEntry{
			ProgramEntry: entry.ProgramEntry,
			isInvalid:    false,
		}
	}

	return &BlockPrograms{
		programs:                  programs,
		latestCommitExecutionTime: ParentBlockTime,
		invalidators:              nil,
	}
}

func (block *BlockPrograms) NextTxIndexForTestingOnly() uint32 {
	return uint32(block.LatestCommitExecutionTimeForTestingOnly()) + 1
}

func (block *BlockPrograms) LatestCommitExecutionTimeForTestingOnly() LogicalTime {
	block.lock.RLock()
	defer block.lock.RUnlock()

	return block.latestCommitExecutionTime
}

func (block *BlockPrograms) EntriesForTestingOnly() map[common.Location]*invalidatableProgramEntry {
	block.lock.RLock()
	defer block.lock.RUnlock()

	entries := make(
		map[common.Location]*invalidatableProgramEntry,
		len(block.programs))
	for locId, entry := range block.programs {
		entries[locId] = entry
	}

	return entries
}

func (block *BlockPrograms) InvalidatorsForTestingOnly() chainedInvalidators {
	block.lock.RLock()
	defer block.lock.RUnlock()

	return block.invalidators
}

func (block *BlockPrograms) GetForTestingOnly(
	location common.Location,
) (
	*interpreter.Program,
	*state.State,
	bool,
) {
	entry := block.get(location)
	if entry != nil {
		return entry.Program, entry.State, true
	}
	return nil, nil, false
}

func (block *BlockPrograms) get(
	location common.Location,
) *invalidatableProgramEntry {
	block.lock.RLock()
	defer block.lock.RUnlock()

	return block.programs[location]
}

func (block *BlockPrograms) unsafeValidate(
	transaction *TransactionPrograms,
) RetryableError {
	if transaction.isSnapshotReadTransaction &&
		transaction.invalidators.ShouldInvalidatePrograms() {

		return newNotRetryableError(
			"invalid TransactionProgram: snapshot read can't invalidate")
	}

	if block.latestCommitExecutionTime >= transaction.executionTime {
		return newNotRetryableError(
			"invalid TransactionProgram: non-increasing time (%v >= %v)",
			block.latestCommitExecutionTime,
			transaction.executionTime)
	}

	if block.latestCommitExecutionTime+1 < transaction.snapshotTime &&
		(!transaction.isSnapshotReadTransaction ||
			transaction.snapshotTime != EndOfBlockExecutionTime) {

		return newNotRetryableError(
			"invalid TransactionProgram: missing commit range [%v, %v)",
			block.latestCommitExecutionTime+1,
			transaction.snapshotTime)
	}

	for _, entry := range transaction.readSet {
		if entry.isInvalid {
			return newRetryableError(
				"invalid TransactionPrograms. outdated read set")
		}
	}

	applicable := block.invalidators.ApplicableInvalidators(
		transaction.snapshotTime)
	if applicable.ShouldInvalidatePrograms() {
		for _, entry := range transaction.writeSet {
			if applicable.ShouldInvalidateEntry(entry) {
				return newRetryableError(
					"invalid TransactionPrograms. outdated write set")
			}
		}
	}

	return nil
}

func (block *BlockPrograms) validate(
	transaction *TransactionPrograms,
) RetryableError {
	block.lock.RLock()
	defer block.lock.RUnlock()

	return block.unsafeValidate(transaction)
}

func (block *BlockPrograms) commit(
	transaction *TransactionPrograms,
) RetryableError {
	block.lock.Lock()
	defer block.lock.Unlock()

	// NOTE: Instead of throwing out all the write entries, we can commit
	// the valid write entries then return error.
	err := block.unsafeValidate(transaction)
	if err != nil {
		return err
	}

	for locId, entry := range transaction.writeSet {
		_, ok := block.programs[locId]
		if ok {
			// A previous transaction already committed an equivalent program
			// entry.  Since both program entry are valid, just reuse the
			// existing one for future transactions.
			continue
		}

		block.programs[locId] = &invalidatableProgramEntry{
			ProgramEntry: entry,
			isInvalid:    false,
		}
	}

	if transaction.invalidators.ShouldInvalidatePrograms() {
		for locId, entry := range block.programs {
			if transaction.invalidators.ShouldInvalidateEntry(
				entry.ProgramEntry) {

				entry.isInvalid = true
				delete(block.programs, locId)
			}
		}

		block.invalidators = append(
			block.invalidators,
			transaction.invalidators...)
	}

	// NOTE: We cannot advance commit time when we encounter a snapshot read
	// (aka script) transaction since these transactions don't generate new
	// snapshots.  It is safe to commit the entries since snapshot read
	// transactions never invalidate entries.
	if !transaction.isSnapshotReadTransaction {
		block.latestCommitExecutionTime = transaction.executionTime
	}
	return nil
}

func (block *BlockPrograms) newTransactionPrograms(
	upperBoundExecutionTime LogicalTime,
	snapshotTime LogicalTime,
	executionTime LogicalTime,
	isSnapshotReadTransaction bool,
) (
	*TransactionPrograms,
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

	return &TransactionPrograms{
		block:                     block,
		snapshotTime:              snapshotTime,
		executionTime:             executionTime,
		readSet:                   map[common.Location]*invalidatableProgramEntry{},
		writeSet:                  map[common.Location]ProgramEntry{},
		nonAddressSet:             map[common.Location]ProgramEntry{},
		isSnapshotReadTransaction: isSnapshotReadTransaction,
	}, nil
}

func (block *BlockPrograms) NewSnapshotReadTransactionPrograms(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*TransactionPrograms,
	error,
) {
	return block.newTransactionPrograms(
		LargestSnapshotReadTransactionExecutionTime,
		snapshotTime,
		executionTime,
		true)
}

func (block *BlockPrograms) NewTransactionPrograms(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*TransactionPrograms,
	error,
) {
	return block.newTransactionPrograms(
		LargestNormalTransactionExecutionTime,
		snapshotTime,
		executionTime,
		false)
}

func (transaction *TransactionPrograms) Get(
	location common.Location,
) (
	*interpreter.Program,
	*state.State,
	bool,
) {
	_, ok := location.(common.AddressLocation)
	if !ok {
		nonAddrEntry, ok := transaction.nonAddressSet[location]
		return nonAddrEntry.Program, nonAddrEntry.State, ok
	}

	writeEntry, ok := transaction.writeSet[location]
	if ok {
		return writeEntry.Program, writeEntry.State, true
	}

	readEntry := transaction.readSet[location]
	if readEntry != nil {
		return readEntry.Program, readEntry.State, true
	}

	readEntry = transaction.block.get(location)
	if readEntry != nil {
		transaction.readSet[location] = readEntry
		return readEntry.Program, readEntry.State, true
	}

	return nil, nil, false
}

func (transaction *TransactionPrograms) Set(
	location common.Location,
	program *interpreter.Program,
	state *state.State,
) {
	addrLoc, ok := location.(common.AddressLocation)
	if !ok {
		transaction.nonAddressSet[location] = ProgramEntry{
			Program: program,
			State:   state,
		}
		return
	}

	transaction.writeSet[location] = ProgramEntry{
		Location: addrLoc,
		Program:  program,
		State:    state,
	}
}

func (transaction *TransactionPrograms) AddInvalidator(
	invalidator Invalidator,
) {
	if invalidator == nil || !invalidator.ShouldInvalidatePrograms() {
		return
	}

	transaction.invalidators = append(
		transaction.invalidators,
		invalidatorAtTime{
			Invalidator:   invalidator,
			executionTime: transaction.executionTime,
		})
}

func (transaction *TransactionPrograms) Validate() RetryableError {
	return transaction.block.validate(transaction)
}

func (transaction *TransactionPrograms) Commit() RetryableError {
	return transaction.block.commit(transaction)
}

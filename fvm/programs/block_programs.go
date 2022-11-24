package programs

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
)

// BlockPrograms is a simple fork-aware OCC database for "caching" programs
// for a particular block.
type BlockPrograms struct {
	*BlockDerivedData[common.AddressLocation, ProgramEntry]
}

// TransactionPrograms is the scratch space for programs of a single transaction.
type TransactionPrograms struct {
	*TransactionDerivedData[common.AddressLocation, ProgramEntry]

	// TODO(patrick): to make non-address programs back in environment package.
	// NOTE: non-address programs are not reusable across transactions, hence
	// they are kept out of the writeSet and the BlockPrograms database.
	nonAddressSet map[common.Location]ProgramEntry
}

func NewEmptyBlockPrograms() *BlockPrograms {
	return &BlockPrograms{
		NewEmptyBlockDerivedData[common.AddressLocation, ProgramEntry](),
	}
}

// This variant is needed by the chunk verifier, which does not start at the
// beginning of the block.
func NewEmptyBlockProgramsWithTransactionOffset(offset uint32) *BlockPrograms {
	return &BlockPrograms{
		NewEmptyBlockDerivedDataWithOffset[common.AddressLocation, ProgramEntry](offset),
	}
}

func (block *BlockPrograms) NewChildBlockPrograms() *BlockPrograms {
	return &BlockPrograms{
		block.NewChildBlockDerivedData(),
	}
}

func (block *BlockPrograms) NewSnapshotReadTransactionPrograms(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*TransactionPrograms,
	error,
) {
	txn, err := block.NewSnapshotReadTransactionDerivedData(
		snapshotTime,
		executionTime)
	if err != nil {
		return nil, err
	}

	return &TransactionPrograms{
		TransactionDerivedData: txn,
		nonAddressSet:          make(map[common.Location]ProgramEntry),
	}, nil
}

func (block *BlockPrograms) NewTransactionPrograms(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*TransactionPrograms,
	error,
) {
	txn, err := block.NewTransactionDerivedData(snapshotTime, executionTime)
	if err != nil {
		return nil, err
	}

	return &TransactionPrograms{
		TransactionDerivedData: txn,
		nonAddressSet:          make(map[common.Location]ProgramEntry),
	}, nil
}

func (transaction *TransactionPrograms) Get(
	location common.Location,
) (
	*interpreter.Program,
	*state.State,
	bool,
) {
	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		nonAddrEntry, ok := transaction.nonAddressSet[location]
		return nonAddrEntry.Program, nonAddrEntry.State, ok
	}

	programEntry := transaction.TransactionDerivedData.Get(addressLocation)
	if programEntry == nil {
		return nil, nil, false
	}

	return programEntry.Program, programEntry.State, true
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

	transaction.TransactionDerivedData.Set(addrLoc, ProgramEntry{
		Location: addrLoc,
		Program:  program,
		State:    state,
	})
}

func (transaction *TransactionPrograms) AddInvalidator(
	invalidator DerivedDataInvalidator[ProgramEntry],
) {
	transaction.TransactionDerivedData.AddInvalidator(invalidator)
}

func (transaction *TransactionPrograms) Validate() RetryableError {
	return transaction.TransactionDerivedData.Validate()
}

func (transaction *TransactionPrograms) Commit() RetryableError {
	return transaction.TransactionDerivedData.Commit()
}

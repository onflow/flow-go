package programs

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go/fvm/state"
)

// BlockPrograms
type BlockPrograms = OCCBlock[common.AddressLocation, ProgramEntry]

// TransactionPrograms is the scratch space for a single transaction.
type TransactionPrograms struct {
	transactionPrograms OCCBlockItem[common.AddressLocation, ProgramEntry]
	// NOTE: non-address programs are not reusable across transactions, hence
	// they are kept out of the writeSet and the BlockPrograms database.
	nonAddressSet map[common.Location]ProgramEntry
}

func NewEmptyBlockPrograms() *BlockPrograms {
	return NewEmptyOCCBlock[common.AddressLocation, ProgramEntry]()
}

// This variant is needed by the chunk verifier, which does not start at the
// beginning of the block.
func NewEmptyBlockProgramsWithTransactionOffset(offset uint32) *BlockPrograms {
	return NewEmptyOCCBlockWithOffset[common.AddressLocation, ProgramEntry](offset)
}

func NewTransactionPrograms(item OCCBlockItem[common.AddressLocation, ProgramEntry]) *TransactionPrograms {
	return &TransactionPrograms{
		transactionPrograms: item,
		nonAddressSet:       make(map[common.Location]ProgramEntry),
	}
}

func (transaction *TransactionPrograms) Get(location common.Location) (*interpreter.Program, *state.State, bool) {
	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		nonAddrEntry, ok := transaction.nonAddressSet[location]
		if !ok {
			return nil, nil, false
		}
		return nonAddrEntry.Program, nonAddrEntry.State, false
	}

	programEntry := transaction.transactionPrograms.Get(addressLocation)
	return programEntry.Program, programEntry.State, true
}
func (transaction *TransactionPrograms) Set(location common.Location, prog *interpreter.Program, state *state.State) {
	addrLoc, ok := location.(common.AddressLocation)
	if !ok {
		transaction.nonAddressSet[location] = ProgramEntry{
			Program: prog,
			State:   state,
		}
		return
	}

	transaction.transactionPrograms.Set(addrLoc, ProgramEntry{
		Program: prog,
		State:   state,
	})
}

func (transaction *TransactionPrograms) AddInvalidator(invalidator OCCInvalidator[ProgramEntry]) {
	transaction.transactionPrograms.AddInvalidator(invalidator)
}

func (transaction *TransactionPrograms) Commit() RetryableError {
	return transaction.transactionPrograms.Commit()
}

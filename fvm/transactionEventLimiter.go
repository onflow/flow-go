package fvm

import (
	"github.com/onflow/flow-go/fvm/state"
)

type TransactionEventLimiter struct {
	maxTotalEventByteSize uint
}

func NewTransactionEventLimiter(totalByteSizeimit uint) *TransactionEventLimiter {
	return &TransactionEventLimiter{maxTotalEventByteSize: totalByteSizeimit}
}

func (l *TransactionEventLimiter) Process(
	vm *VirtualMachine,
	ctx Context,
	proc *TransactionProcedure,
	ledger state.Ledger,
) error {

	// we don't care about the tx index here
	fEvents := proc.Events
	totalByteSize := uint(0)
	for _, f := range fEvents {
		totalByteSize += f.ByteSize()
	}
	if totalByteSize > l.maxTotalEventByteSize {
		return &MaxEventSizeLimitExceededError{
			TotalByteSize: uint64(totalByteSize),
			Limit:         uint64(l.maxTotalEventByteSize),
		}
	}
	return nil
}

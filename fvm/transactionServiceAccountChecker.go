package fvm

import (
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
)

// TransactionServiceAccountChecker Is responsible for applying any service account exceptions to the transaction.
// This should be run after signature verification so that we are sure it is the service account that is the payer,
// and not someone claiming to be the service account.
type TransactionServiceAccountChecker struct{}

// NewTransactionServiceAccountChecker creates a new TransactionServiceAccountChecker
func NewTransactionServiceAccountChecker() *TransactionServiceAccountChecker {
	return &TransactionServiceAccountChecker{}
}

// Process Checks if the transaction payer is the service account.
// If it is it applies some exceptions to metering.
func (c *TransactionServiceAccountChecker) Process(
	_ *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	_ *programs.Programs,
) error {

	if proc.Transaction.Payer == ctx.Chain.ServiceAddress() {
		sth.SetPayerIsServiceAccount()
	}
	return nil
}

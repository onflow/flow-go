package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionAccountFrozenChecker struct{}

func NewTransactionAccountFrozenChecker() *TransactionAccountFrozenChecker {
	return &TransactionAccountFrozenChecker{}
}

func (c *TransactionAccountFrozenChecker) Process(
	_ *VirtualMachine,
	_ *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	_ *programs.Programs,
) (txError error, vmError error) {
	return c.checkAccountNotFrozen(proc.Transaction, sth)
}

func (c *TransactionAccountFrozenChecker) checkAccountNotFrozen(
	tx *flow.TransactionBody,
	sth *state.StateHolder,
) (txError error, vmError error) {
	accounts := state.NewAccounts(sth)

	for _, authorizer := range tx.Authorizers {
		err := accounts.CheckAccountNotFrozen(authorizer)
		if err != nil {
			// TODO diff between txError and vmError
			return fmt.Errorf("check account not frozen authorizer: %w", err), nil
		}
	}

	err := accounts.CheckAccountNotFrozen(tx.ProposalKey.Address)
	if err != nil {
		// TODO diff between txError and vmError
		return fmt.Errorf("check account not frozen proposal: %w", err), nil
	}

	err = accounts.CheckAccountNotFrozen(tx.Payer)
	if err != nil {
		// TODO diff between txError and vmError
		return fmt.Errorf("check account not frozen payer: %w", err), nil
	}

	return nil, nil
}

type TransactionAccountFrozenEnabler struct{}

func NewTransactionAccountFrozenEnabler() *TransactionAccountFrozenEnabler {
	return &TransactionAccountFrozenEnabler{}
}

func (c *TransactionAccountFrozenEnabler) Process(
	_ *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	_ *state.StateHolder,
	_ *programs.Programs,
) (txError error, vmError error) {

	serviceAddress := ctx.Chain.ServiceAddress()

	for _, signature := range proc.Transaction.EnvelopeSignatures {
		if signature.Address == serviceAddress {
			ctx.AccountFreezeAvailable = true
			return nil, nil //we can bail out and save maybe some loops
		}
	}

	return nil, nil
}

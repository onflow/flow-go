package fvm

import (
	"fmt"

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
	st *state.State,
	programs *Programs,
) error {
	return c.checkAccountNotFrozen(proc.Transaction, st)
}

func (c *TransactionAccountFrozenChecker) checkAccountNotFrozen(
	tx *flow.TransactionBody,
	st *state.State,
) error {
	accounts := state.NewAccounts(st)

	for _, authorizer := range tx.Authorizers {
		err := accounts.CheckAccountNotFrozen(authorizer)
		if err != nil {
			return fmt.Errorf("check account not frozen authorizer: %w", err)
		}
	}

	err := accounts.CheckAccountNotFrozen(tx.ProposalKey.Address)
	if err != nil {
		return fmt.Errorf("check account not frozen proposal: %w", err)
	}

	err = accounts.CheckAccountNotFrozen(tx.Payer)
	if err != nil {
		return fmt.Errorf("check account not frozen payer: %w", err)
	}

	return nil
}

type TransactionAccountFrozenEnabler struct{}

func NewTransactionAccountFrozenEnabler() *TransactionAccountFrozenEnabler {
	return &TransactionAccountFrozenEnabler{}
}

func (c *TransactionAccountFrozenEnabler) Process(
	_ *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	_ *state.State,
	_ *Programs,
) error {

	serviceAddress := ctx.Chain.ServiceAddress()

	for _, signature := range proc.Transaction.EnvelopeSignatures {
		if signature.Address == serviceAddress {
			ctx.AccountFreezeAvailable = true
			return nil //we can bail out and save maybe some loops
		}
	}

	return nil
}

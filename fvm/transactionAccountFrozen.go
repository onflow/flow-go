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
) error {
	sth.DisableAllLimitEnforcements()
	defer sth.EnableAllLimitEnforcements()
	return c.checkAccountNotFrozen(proc.Transaction, sth)
}

func (c *TransactionAccountFrozenChecker) checkAccountNotFrozen(
	tx *flow.TransactionBody,
	sth *state.StateHolder,
) error {
	accounts := state.NewAccounts(sth)

	for _, authorizer := range tx.Authorizers {
		err := accounts.CheckAccountNotFrozen(authorizer)
		if err != nil {
			return fmt.Errorf("checking frozen account failed: %w", err)
		}
	}

	err := accounts.CheckAccountNotFrozen(tx.ProposalKey.Address)
	if err != nil {
		return fmt.Errorf("checking frozen account failed: %w", err)
	}

	err = accounts.CheckAccountNotFrozen(tx.Payer)
	if err != nil {
		return fmt.Errorf("checking frozen account failed: %w", err)
	}

	return nil
}

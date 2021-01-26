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
	_ Context,
	proc *TransactionProcedure,
	st *state.State,
) error {

	return c.checkAccountNotFrozen(proc.Transaction, st)
}

func (c *TransactionAccountFrozenChecker) checkAccountNotFrozen(
	tx *flow.TransactionBody,
	st *state.State,
) error {
	accounts := state.NewAccounts(st)

	errIfFrozen := func(address flow.Address) error {
		frozen, err := accounts.GetAccountFrozen(address)
		if err != nil {
			return fmt.Errorf("cannot check acount free status: %w", err)
		}
		if frozen {
			return &AccountFrozenError{Address: address}
		}
		return nil
	}

	for _, authorizer := range tx.Authorizers {
		err := errIfFrozen(authorizer)
		if err != nil {
			return err
		}
	}

	err := errIfFrozen(tx.ProposalKey.Address)
	if err != nil {
		return err
	}

	return errIfFrozen(tx.Payer)
}

package fvm

import (
	"errors"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionAccountFrozenChecker struct{}

func NewTransactionAccountFrozenChecker() *TransactionAccountFrozenChecker {
	return &TransactionAccountFrozenChecker{}
}

func (c *TransactionAccountFrozenChecker) Process(
	vm *VirtualMachine,
	ctx Context,
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

	//accounts.GetAccountFrozen(tx.Authorizers)

	proposalKey := tx.ProposalKey

	accountKey, err := accounts.GetPublicKey(proposalKey.Address, proposalKey.KeyIndex)
	if err != nil {
		if errors.Is(err, state.ErrAccountPublicKeyNotFound) {
			return &InvalidProposalKeyPublicKeyDoesNotExistError{
				Address:  proposalKey.Address,
				KeyIndex: proposalKey.KeyIndex,
			}
		}

		return err
	}

	if accountKey.Revoked {
		return &InvalidProposalKeyPublicKeyRevokedError{
			Address:  proposalKey.Address,
			KeyIndex: proposalKey.KeyIndex,
		}
	}

	valid := accountKey.SeqNumber == proposalKey.SequenceNumber

	if !valid {
		return &InvalidProposalKeySequenceNumberError{
			Address:           proposalKey.Address,
			KeyIndex:          proposalKey.KeyIndex,
			CurrentSeqNumber:  accountKey.SeqNumber,
			ProvidedSeqNumber: proposalKey.SequenceNumber,
		}
	}

	accountKey.SeqNumber++

	_, err = accounts.SetPublicKey(proposalKey.Address, proposalKey.KeyIndex, accountKey)
	if err != nil {
		return err
	}

	return nil
}

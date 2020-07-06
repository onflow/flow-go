package fvm

import (
	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

type TransactionSequenceNumberChecker struct{}

func NewTransactionSequenceNumberChecker() *TransactionSequenceNumberChecker {
	return &TransactionSequenceNumberChecker{}
}

func (c *TransactionSequenceNumberChecker) Process(
	vm *VirtualMachine,
	ctx Context,
	proc *TransactionProcedure,
	ledger state.Ledger,
) error {
	addresses := state.NewAddresses(ledger, ctx.Chain)
	accounts := state.NewAccounts(ledger, addresses)

	return c.checkAndIncrementSequenceNumber(proc.Transaction, accounts)
}

func (c *TransactionSequenceNumberChecker) checkAndIncrementSequenceNumber(
	tx *flow.TransactionBody,
	accounts *state.Accounts,
) error {
	proposalKey := tx.ProposalKey

	accountKeys, err := accounts.GetPublicKeys(proposalKey.Address)
	if err != nil {
		return err
	}

	if int(proposalKey.KeyID) >= len(accountKeys) {
		return &ProposalKeyDoesNotExistError{
			Address: proposalKey.Address,
			KeyID:   proposalKey.KeyID,
		}
	}

	accountKey := accountKeys[proposalKey.KeyID]

	valid := accountKey.SeqNumber == proposalKey.SequenceNumber

	if !valid {
		return &InvalidProposalKeySequenceNumberError{
			Address:           proposalKey.Address,
			KeyID:             proposalKey.KeyID,
			CurrentSeqNumber:  accountKey.SeqNumber,
			ProvidedSeqNumber: proposalKey.SequenceNumber,
		}
	}

	accountKey.SeqNumber++

	var updatedAccountKeyBytes []byte
	updatedAccountKeyBytes, err = flow.EncodeAccountPublicKey(accountKey)
	if err != nil {
		return err
	}

	accounts.SetPublicKey(proposalKey.Address, proposalKey.KeyID, updatedAccountKeyBytes)

	return nil
}

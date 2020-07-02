package fvm

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type TransactionSequenceNumberChecker struct{}

func NewTransactionSequenceNumberChecker() *TransactionSequenceNumberChecker {
	return &TransactionSequenceNumberChecker{}
}

func (c *TransactionSequenceNumberChecker) Process(
	vm *VirtualMachine,
	ctx Context,
	inv *InvokableTransaction,
	ledger Ledger,
) error {
	return c.checkAndIncrementSequenceNumber(inv.Transaction, ledger)
}

func (c *TransactionSequenceNumberChecker) checkAndIncrementSequenceNumber(
	tx *flow.TransactionBody,
	ledger Ledger,
) error {
	proposalKey := tx.ProposalKey

	accountKeys, err := getAccountPublicKeys(ledger, proposalKey.Address)
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

	setAccountPublicKey(ledger, proposalKey.Address, proposalKey.KeyID, updatedAccountKeyBytes)

	return nil
}

package processor

import (
	"errors"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionSequenceNumberChecker struct{}

func NewTransactionSequenceNumberChecker() *TransactionSequenceNumberChecker {
	return &TransactionSequenceNumberChecker{}
}

func (c *TransactionSequenceNumberChecker) Process(
	vm *fvm.VirtualMachine,
	proc *fvm.TransactionProcedure,
	env *fvm.Environment,
) error {

	return c.checkAndIncrementSequenceNumber(proc.Transaction, env)
}

func (c *TransactionSequenceNumberChecker) checkAndIncrementSequenceNumber(
	tx *flow.TransactionBody,
	env *fvm.Environment,
) error {
	accounts := env.Accounts()
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

	err = st.Commit()
	if err != nil {
		return err
	}

	return nil
}

package fvm

import (
	"errors"

	"github.com/opentracing/opentracing-go/log"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionSequenceNumberChecker struct{}

func NewTransactionSequenceNumberChecker() *TransactionSequenceNumberChecker {
	return &TransactionSequenceNumberChecker{}
}

func (c *TransactionSequenceNumberChecker) Process(
	vm *VirtualMachine,
	ctx Context,
	proc *TransactionProcedure,
	st *state.State,
	programs *Programs,
) error {

	return c.checkAndIncrementSequenceNumber(proc, ctx, st)
}

func (c *TransactionSequenceNumberChecker) checkAndIncrementSequenceNumber(
	proc *TransactionProcedure,
	ctx Context,
	st *state.State,
) error {

	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span := ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMSeqNumCheckTransaction)
		span.LogFields(
			log.String("transaction.ID", proc.ID.String()),
		)
		defer span.Finish()
	}

	accounts := state.NewAccounts(st)
	proposalKey := proc.Transaction.ProposalKey

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

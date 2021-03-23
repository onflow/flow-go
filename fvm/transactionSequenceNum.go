package fvm

import (
	"fmt"

	"github.com/opentracing/opentracing-go/log"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionSequenceNumberChecker struct{}

func NewTransactionSequenceNumberChecker() *TransactionSequenceNumberChecker {
	return &TransactionSequenceNumberChecker{}
}

func (c *TransactionSequenceNumberChecker) Process(
	_ *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) error {
	return c.checkAndIncrementSequenceNumber(proc, ctx, sth)
}

func (c *TransactionSequenceNumberChecker) checkAndIncrementSequenceNumber(
	proc *TransactionProcedure,
	ctx *Context,
	sth *state.StateHolder,
) error {

	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span := ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMSeqNumCheckTransaction)
		span.LogFields(
			log.String("transaction.ID", proc.ID.String()),
		)
		defer span.Finish()
	}

	parentState := sth.State()
	childState := sth.NewChild()
	defer func() {
		if mergeError := parentState.MergeState(childState); mergeError != nil {
			panic(mergeError)
		}
		sth.SetActiveState(parentState)
	}()

	accounts := state.NewAccounts(sth)
	proposalKey := proc.Transaction.ProposalKey

	accountKey, err := accounts.GetPublicKey(proposalKey.Address, proposalKey.KeyIndex)
	if err != nil {
		issue := &errors.InvalidProposalSignatureError{
			Address:  proposalKey.Address,
			KeyIndex: proposalKey.KeyIndex,
			Err:      err,
		}
		return fmt.Errorf("checking sequence number failed: %w", issue)
	}

	if accountKey.Revoked {
		issue := &errors.InvalidProposalSignatureError{
			Address:  proposalKey.Address,
			KeyIndex: proposalKey.KeyIndex,
			Err:      fmt.Errorf("proposal key has been revoked"),
		}
		return fmt.Errorf("checking sequence number failed: %w", issue)
	}

	valid := accountKey.SeqNumber == proposalKey.SequenceNumber

	if !valid {
		return &errors.ProposalSeqNumberMismatchError{
			Address:           proposalKey.Address,
			KeyIndex:          proposalKey.KeyIndex,
			CurrentSeqNumber:  accountKey.SeqNumber,
			ProvidedSeqNumber: proposalKey.SequenceNumber,
		}
	}

	accountKey.SeqNumber++

	_, err = accounts.SetPublicKey(proposalKey.Address, proposalKey.KeyIndex, accountKey)
	if err != nil {
		childState.View().DropDelta()
		return fmt.Errorf("checking sequence number failed: %w", err)
	}
	return nil
}

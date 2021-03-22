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
) (txError errors.TransactionError, vmError errors.VMError) {
	return c.checkAndIncrementSequenceNumber(proc, ctx, sth)
}

func (c *TransactionSequenceNumberChecker) checkAndIncrementSequenceNumber(
	proc *TransactionProcedure,
	ctx *Context,
	sth *state.StateHolder,
) (txError errors.TransactionError, vmError errors.VMError) {

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
		txError, vmError = errors.SplitErrorTypes(err)
		if vmError != nil {
			return nil, vmError
		}
		if txError != nil {
			return &errors.InvalidProposalSignatureError{
				Address:  proposalKey.Address,
				KeyIndex: proposalKey.KeyIndex,
				Err:      err,
			}, nil
		}
	}

	if accountKey.Revoked {
		return &errors.InvalidProposalSignatureError{
			Address:  proposalKey.Address,
			KeyIndex: proposalKey.KeyIndex,
			Err:      fmt.Errorf("proposal key has been revoked"),
		}, nil
	}

	valid := accountKey.SeqNumber == proposalKey.SequenceNumber

	if !valid {
		return &errors.ProposalSeqNumberMismatchError{
			Address:           proposalKey.Address,
			KeyIndex:          proposalKey.KeyIndex,
			CurrentSeqNumber:  accountKey.SeqNumber,
			ProvidedSeqNumber: proposalKey.SequenceNumber,
		}, nil
	}

	accountKey.SeqNumber++

	_, err = accounts.SetPublicKey(proposalKey.Address, proposalKey.KeyIndex, accountKey)
	if err != nil {
		txError, vmError = errors.SplitErrorTypes(err)
		if vmError != nil {
			return nil, vmError
		}
		if txError != nil {
			// drop the changes
			childState.View().DropDelta()
			return txError, nil
		}
	}
	return nil, nil
}

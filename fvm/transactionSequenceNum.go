package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionSequenceNumberChecker struct{}

func NewTransactionSequenceNumberChecker() *TransactionSequenceNumberChecker {
	return &TransactionSequenceNumberChecker{}
}

func (c *TransactionSequenceNumberChecker) NewExecutor(
	ctx Context,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	_ *programs.TransactionPrograms,
) TransactionExecutor {
	return newSequenceNumberCheckExecutor(ctx, proc, txnState)
}

func (c *TransactionSequenceNumberChecker) Process(
	ctx Context,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	txnPrograms *programs.TransactionPrograms,
) error {
	return run(c.NewExecutor(ctx, proc, txnState, txnPrograms))
}

type sequenceNumberCheckExecutor struct {
	proc     *TransactionProcedure
	txnState *state.TransactionState

	tracer module.Tracer
}

func newSequenceNumberCheckExecutor(
	ctx Context,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
) *sequenceNumberCheckExecutor {
	return &sequenceNumberCheckExecutor{
		proc:     proc,
		txnState: txnState,
		tracer:   ctx.Tracer,
	}
}

func (*sequenceNumberCheckExecutor) Preprocess() error {
	// Does nothing.
	return nil
}

func (*sequenceNumberCheckExecutor) Cleanup() {
	// Does nothing.
}

func (executor *sequenceNumberCheckExecutor) Execute() error {
	// TODO(Janez): verification is part of inclusion fees, not execution fees.
	var err error
	executor.txnState.RunWithAllLimitsDisabled(func() {
		err = executor.checkAndIncrementSequenceNumber()
	})

	if err != nil {
		return fmt.Errorf("checking sequence number failed: %w", err)
	}

	return nil
}

func (executor *sequenceNumberCheckExecutor) checkAndIncrementSequenceNumber() error {

	defer executor.proc.StartSpanFromProcTraceSpan(
		executor.tracer,
		trace.FVMSeqNumCheckTransaction).End()

	nestedTxnId, err := executor.txnState.BeginNestedTransaction()
	if err != nil {
		return err
	}

	defer func() {
		_, commitError := executor.txnState.Commit(nestedTxnId)
		if commitError != nil {
			panic(commitError)
		}
	}()

	accounts := environment.NewAccounts(executor.txnState)
	proposalKey := executor.proc.Transaction.ProposalKey

	var accountKey flow.AccountPublicKey

	accountKey, err = accounts.GetPublicKey(
		proposalKey.Address,
		proposalKey.KeyIndex)
	if err != nil {
		return errors.NewInvalidProposalSignatureError(proposalKey, err)
	}

	if accountKey.Revoked {
		return errors.NewInvalidProposalSignatureError(
			proposalKey,
			fmt.Errorf("proposal key has been revoked"))
	}

	// Note that proposal key verification happens at the txVerifier and not here.
	valid := accountKey.SeqNumber == proposalKey.SequenceNumber

	if !valid {
		return errors.NewInvalidProposalSeqNumberError(
			proposalKey,
			accountKey.SeqNumber)
	}

	accountKey.SeqNumber++

	_, err = accounts.SetPublicKey(
		proposalKey.Address,
		proposalKey.KeyIndex,
		accountKey)
	if err != nil {
		restartError := executor.txnState.RestartNestedTransaction(nestedTxnId)
		if restartError != nil {
			panic(restartError)
		}
		return err
	}

	return nil
}

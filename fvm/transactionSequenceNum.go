package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionSequenceNumberChecker struct{}

func NewTransactionSequenceNumberChecker() *TransactionSequenceNumberChecker {
	return &TransactionSequenceNumberChecker{}
}

func (c *TransactionSequenceNumberChecker) Process(
	ctx Context,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	_ *programs.TransactionPrograms,
) error {
	err := c.checkAndIncrementSequenceNumber(proc, ctx, txnState)
	if err != nil {
		return fmt.Errorf("checking sequence number failed: %w", err)
	}

	return nil
}

func (c *TransactionSequenceNumberChecker) checkAndIncrementSequenceNumber(
	proc *TransactionProcedure,
	ctx Context,
	txnState *state.TransactionState,
) error {

	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span := ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMSeqNumCheckTransaction)
		defer span.End()
	}

	nestedTxnId, err := txnState.BeginNestedTransaction()
	if err != nil {
		return err
	}

	defer func() {
		// Skip checking limits when merging the public key sequence number
		txnState.RunWithAllLimitsDisabled(func() {
			mergeError := txnState.Commit(nestedTxnId)
			if mergeError != nil {
				panic(mergeError)
			}
		})
	}()

	accounts := environment.NewAccounts(txnState)
	proposalKey := proc.Transaction.ProposalKey

	var accountKey flow.AccountPublicKey

	// TODO(Janez): move disabling limits out of the sequence number verifier. Verifier should not be metered anyway.
	// TODO(Janez): verification is part of inclusion fees, not execution fees.

	// Skip checking limits when getting the public key
	txnState.RunWithAllLimitsDisabled(func() {
		accountKey, err = accounts.GetPublicKey(proposalKey.Address, proposalKey.KeyIndex)
	})
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
		return errors.NewInvalidProposalSeqNumberError(proposalKey, accountKey.SeqNumber)
	}

	accountKey.SeqNumber++

	// Skip checking limits when setting the public key sequence number
	txnState.RunWithAllLimitsDisabled(func() {
		_, err = accounts.SetPublicKey(proposalKey.Address, proposalKey.KeyIndex, accountKey)
	})
	if err != nil {
		// NOTE: we need to disable limits during restart or else restart may
		// fail on merging.
		txnState.RunWithAllLimitsDisabled(func() {
			_ = txnState.RestartNestedTransaction(nestedTxnId)
		})
		return err
	}

	return nil
}

package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionSequenceNumberChecker struct{}

func (c TransactionSequenceNumberChecker) CheckAndIncrementSequenceNumber(
	tracer tracing.TracerSpan,
	proc *TransactionProcedure,
	txnState storage.TransactionPreparer,
) error {
	// TODO(Janez): verification is part of inclusion fees, not execution fees.
	var err error
	txnState.RunWithAllLimitsDisabled(func() {
		err = c.checkAndIncrementSequenceNumber(tracer, proc, txnState)
	})

	if err != nil {
		return fmt.Errorf("checking sequence number failed: %w", err)
	}

	return nil
}

func (c TransactionSequenceNumberChecker) checkAndIncrementSequenceNumber(
	tracer tracing.TracerSpan,
	proc *TransactionProcedure,
	txnState storage.TransactionPreparer,
) error {

	defer tracer.StartChildSpan(trace.FVMSeqNumCheckTransaction).End()

	nestedTxnId, err := txnState.BeginNestedTransaction()
	if err != nil {
		return err
	}

	defer func() {
		_, commitError := txnState.CommitNestedTransaction(nestedTxnId)
		if commitError != nil {
			panic(commitError)
		}
	}()

	accounts := environment.NewAccounts(txnState)
	proposalKey := proc.Transaction.ProposalKey

	var accountKey flow.AccountPublicKey

	accountKey, err = accounts.GetPublicKey(proposalKey.Address, proposalKey.KeyIndex)
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

	_, err = accounts.SetPublicKey(proposalKey.Address, proposalKey.KeyIndex, accountKey)
	if err != nil {
		restartError := txnState.RestartNestedTransaction(nestedTxnId)
		if restartError != nil {
			panic(restartError)
		}
		return err
	}

	return nil
}

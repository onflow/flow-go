package fvm

import (
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
)

type TransactionPayerBalanceChecker struct{}

func (_ TransactionPayerBalanceChecker) CheckPayerBalanceAndReturnMaxFees(
	proc *TransactionProcedure,
	txnState storage.TransactionPreparer,
	env environment.Environment,
) (uint64, error) {
	if !env.TransactionFeesEnabled() {
		// if the transaction fees are not enabled,
		// the payer can pay for the transaction,
		// and the max transaction fees are 0.
		// This is also the condition that gets hit during bootstrapping.
		return 0, nil
	}

	var resultValue cadence.Value
	var err error
	txnState.RunWithAllLimitsDisabled(func() {
		// Don't meter the payer balance check.
		// It has a static cost, and its cost should be part of the inclusion fees, not part of execution fees.
		resultValue, err = env.CheckPayerBalanceAndGetMaxTxFees(
			proc.Transaction.Payer,
			proc.Transaction.InclusionEffort(),
			uint64(txnState.TotalComputationLimit()),
		)
	})
	if err != nil {
		return 0, errors.NewPayerBalanceCheckFailure(proc.Transaction.Payer, err)
	}

	// parse expected result from the Cadence runtime
	// https://github.com/onflow/flow-core-contracts/blob/7c70c6a1d33c2879b60c78e363fa68fc6fce13b9/contracts/FlowFees.cdc#L75
	result, ok := resultValue.(cadence.Struct)
	if ok && len(result.Fields) == 3 {
		payerCanPay, okBool := result.Fields[0].(cadence.Bool)
		requiredBalance, okBalance := result.Fields[1].(cadence.UFix64)
		maxFees, okFees := result.Fields[2].(cadence.UFix64)

		if okBool && okBalance && okFees {
			if !payerCanPay {
				return 0, errors.NewInsufficientPayerBalanceError(proc.Transaction.Payer, requiredBalance)
			}
			return uint64(maxFees), nil
		}
	}

	return 0, errors.NewPayerBalanceCheckFailure(proc.Transaction.Payer, fmt.Errorf("invalid result type"))
}

package fvm

import (
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
)

type TransactionPayerBalanceChecker struct{}

const VerifyPayerBalanceResultTypeCanExecuteTransactionFieldName = "canExecuteTransaction"
const VerifyPayerBalanceResultTypeRequiredBalanceFieldName = "requiredBalance"
const VerifyPayerBalanceResultTypeMaximumTransactionFeesFieldName = "maximumTransactionFees"

// decodeVerifyPayerBalanceResult decodes the VerifyPayerBalanceResult struct
// https://github.com/onflow/flow-core-contracts/blob/7c70c6a1d33c2879b60c78e363fa68fc6fce13b9/contracts/FlowFees.cdc#L75
func decodeVerifyPayerBalanceResult(resultValue cadence.Value) (
	canExecuteTransaction cadence.Bool,
	requiredBalance cadence.UFix64,
	maximumTransactionFees cadence.UFix64,
	err error,
) {
	result, ok := resultValue.(cadence.Struct)
	if !ok {
		return false, 0, 0, fmt.Errorf("invalid VerifyPayerBalanceResult value: not a struct")
	}

	fields := cadence.FieldsMappedByName(result)

	canExecuteTransaction, ok = fields[VerifyPayerBalanceResultTypeCanExecuteTransactionFieldName].(cadence.Bool)
	if !ok {
		return false, 0, 0, fmt.Errorf(
			"invalid VerifyPayerBalanceResult field: %s",
			VerifyPayerBalanceResultTypeCanExecuteTransactionFieldName,
		)
	}

	requiredBalance, ok = fields[VerifyPayerBalanceResultTypeRequiredBalanceFieldName].(cadence.UFix64)
	if !ok {
		return false, 0, 0, fmt.Errorf(
			"invalid VerifyPayerBalanceResult field: %s",
			VerifyPayerBalanceResultTypeRequiredBalanceFieldName,
		)
	}

	maximumTransactionFees, ok = fields[VerifyPayerBalanceResultTypeMaximumTransactionFeesFieldName].(cadence.UFix64)
	if !ok {
		return false, 0, 0, fmt.Errorf(
			"invalid VerifyPayerBalanceResult field: %s",
			VerifyPayerBalanceResultTypeMaximumTransactionFeesFieldName,
		)
	}

	return canExecuteTransaction, requiredBalance, maximumTransactionFees, nil
}

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

	payerCanPay, requiredBalance, maxFees, err := decodeVerifyPayerBalanceResult(resultValue)
	if err != nil {
		return 0, errors.NewPayerBalanceCheckFailure(proc.Transaction.Payer, err)
	}

	if !payerCanPay {
		return 0, errors.NewInsufficientPayerBalanceError(proc.Transaction.Payer, requiredBalance)
	}

	return uint64(maxFees), nil
}

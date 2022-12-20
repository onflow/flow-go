package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionStorageLimiter struct{}

// CheckStorageLimits checks each account that had its storage written to during a transaction, that its storage used
// is less than its storage capacity.
// Storage used is an FVM register and is easily accessible.
// Storage capacity is calculated by the FlowStorageFees contract from the account's flow balance.
//
// The payers balance is considered to be maxTxFees lower that its actual balance, due to the fact that
// the fee deduction step happens after the storage limit check.
func (d TransactionStorageLimiter) CheckStorageLimits(
	env environment.Environment,
	addresses []flow.Address,
	payer flow.Address,
	maxTxFees uint64,
) error {
	if !env.LimitAccountStorage() {
		return nil
	}

	defer env.StartSpanFromRoot(trace.FVMTransactionStorageUsedCheck).End()

	err := d.checkStorageLimits(env, addresses, payer, maxTxFees)
	if err != nil {
		return fmt.Errorf("storage limit check failed: %w", err)
	}
	return nil
}

func (d TransactionStorageLimiter) checkStorageLimits(
	env environment.Environment,
	addresses []flow.Address,
	payer flow.Address,
	maxTxFees uint64,
) error {
	// in case the payer is not already part of the check, include it here.
	// If the maxTxFees is zero, it doesn't matter if the payer is included or not.
	if maxTxFees > 0 {
		commonAddressesContainPayer := false
		for _, address := range addresses {
			if address == payer {
				commonAddressesContainPayer = true
				break
			}
		}

		if !commonAddressesContainPayer {
			addresses = append(addresses, payer)
		}
	}

	commonAddresses := make([]common.Address, len(addresses))
	usages := make([]uint64, len(commonAddresses))

	for i, address := range addresses {
		ca := common.Address(address)
		u, err := env.GetStorageUsed(ca)
		if err != nil {
			return err
		}

		commonAddresses[i] = ca
		usages[i] = u
	}

	result, invokeErr := env.AccountsStorageCapacity(
		commonAddresses,
		common.Address(payer),
		maxTxFees,
	)

	// This error only occurs in case of implementation errors. The InvokeAccountsStorageCapacity
	// already handles cases where the default vault is missing.
	if invokeErr != nil {
		return invokeErr
	}

	// the resultArray elements are in the same order as the addresses and the addresses are deterministically sorted
	resultArray, ok := result.(cadence.Array)
	if !ok {
		return fmt.Errorf("AccountsStorageCapacity did not return an array")
	}

	for i, value := range resultArray.Values {
		capacity := environment.StorageMBUFixToBytesUInt(value)

		if usages[i] > capacity {
			return errors.NewStorageCapacityExceededError(flow.BytesToAddress(addresses[i].Bytes()), usages[i], capacity)
		}
	}

	return nil
}

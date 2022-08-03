package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionStorageLimiter struct{}

func NewTransactionStorageLimiter() TransactionStorageLimiter {
	return TransactionStorageLimiter{}
}

func (d TransactionStorageLimiter) CheckLimits(
	env Environment,
	addresses []flow.Address,
) error {
	if !env.Context().LimitAccountStorage {
		return nil
	}

	defer env.StartSpanFromRoot(trace.FVMTransactionStorageUsedCheck).End()

	commonAddresses := make([]common.Address, len(addresses))
	usages := make([]uint64, len(commonAddresses))
	for i, address := range addresses {
		c := common.Address(address)
		commonAddresses[i] = c
		u, err := env.GetStorageUsed(c)
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}
		usages[i] = u
	}

	result, invokeErr := InvokeAccountsStorageCapacity(env, commonAddresses)

	// This error only occurs in case of implementation errors. The InvokeAccountsStorageCapacity
	// already handles cases where the default vault is missing.
	if invokeErr != nil {
		return invokeErr
	}

	// the resultArray elements are in the same order as the addresses and the addresses are deterministically sorted
	resultArray, ok := result.(cadence.Array)
	if !ok {
		return fmt.Errorf("storage limit check failed: AccountsStorageCapacity did not return an array")
	}

	for i, value := range resultArray.Values {
		capacity := storageMBUFixToBytesUInt(value)

		if usages[i] > capacity {
			return errors.NewStorageCapacityExceededError(flow.BytesToAddress(addresses[i].Bytes()), usages[i], capacity)
		}
	}

	return nil
}

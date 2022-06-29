package fvm

import (
	"fmt"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go/fvm/meter"

	"github.com/onflow/cadence/runtime/common"

	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionStorageLimiter struct{}

func NewTransactionStorageLimiter() *TransactionStorageLimiter {
	return &TransactionStorageLimiter{}
}

func (d *TransactionStorageLimiter) CheckLimits(env *TransactionEnv, addresses []flow.Address, inter *interpreter.Interpreter) error {
	if !env.Context().LimitAccountStorage {
		return nil
	}

	// iterating through a map in a non-deterministic order! Do not exit the loop early.
	for _, address := range addresses {
		commonAddress := common.Address(address)

		err := env.meterComputation(meter.ComputationKindGetStorageCapacity, 1)
		if err != nil {
			return fmt.Errorf("get storage capacity failed: %w", err)
		}

		accountStorageCapacity := AccountStorageCapacityInvocation(env, env.traceSpan, inter)
		result, invokeErr := accountStorageCapacity(commonAddress)

		// TODO: Figure out how to handle this error. Currently if a runtime error occurs, storage capacity will be 0.
		// 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
		// 2. There will also be an error in case the accounts balance times megabytesPerFlow constant overflows,
		//		which shouldn't happen unless the the price of storage is reduced at least 100 fold
		// 3. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
		if invokeErr != nil {
			return invokeErr
		}

		capacity := storageMBUFixToBytesUInt(result)

		usage, err := env.GetStorageUsed(commonAddress)
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}

		if usage > capacity {
			return errors.NewStorageCapacityExceededError(address, usage, capacity)
		}
	}

	return nil
}

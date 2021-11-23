package fvm

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionStorageLimiter struct{}

func NewTransactionStorageLimiter() *TransactionStorageLimiter {
	return &TransactionStorageLimiter{}
}

func (d *TransactionStorageLimiter) CheckLimits(
	env Environment,
	addresses []flow.Address,
) error {
	if !env.Context().LimitAccountStorage {
		return nil
	}

	// iterating through a map in a non-deterministic order! Do not exit the loop early.
	for _, address := range addresses {
		commonAddress := common.Address(address)

		capacity, err := env.GetStorageCapacity(commonAddress)
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}

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

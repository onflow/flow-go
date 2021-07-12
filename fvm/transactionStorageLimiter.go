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
	env *hostEnv,
	addresses []flow.Address,
) error {
	if !env.ctx.LimitAccountStorage {
		return nil
	}

	for _, address := range addresses {
		add := common.BytesToAddress(address.Bytes())

		capacity, err := env.GetStorageCapacity(add)
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}

		usage, err := env.GetStorageUsed(add)
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}

		if usage > capacity {
			return errors.NewStorageCapacityExceededError(address, usage, capacity)
		}
	}
	return nil
}

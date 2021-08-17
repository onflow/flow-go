package fvm

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/onflow/cadence/runtime/common"

	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionStorageLimiter struct{}

func NewTransactionStorageLimiter() *TransactionStorageLimiter {
	return &TransactionStorageLimiter{}
}

func (d *TransactionStorageLimiter) CheckLimits(
	env Enviornment,
	addresses map[flow.Address]struct{},
) (err error) {
	if !env.Context().LimitAccountStorage {
		return nil
	}

	// iterating through a map in a non-deterministic order! Do not exit the loop early.
	for address := range addresses {
		commonAddress := common.BytesToAddress(address.Bytes())

		capacity, aerr := env.GetStorageCapacity(commonAddress)
		if aerr != nil {
			aerr = fmt.Errorf("storage limit check failed: %w", aerr)
			err = multierror.Append(err, aerr)
			continue
		}

		usage, aerr := env.GetStorageUsed(commonAddress)
		if aerr != nil {
			aerr = fmt.Errorf("storage limit check failed: %w", aerr)
			err = multierror.Append(err, aerr)
			continue
		}

		if usage > capacity {
			err = multierror.Append(err, errors.NewStorageCapacityExceededError(address, usage, capacity))
			continue
		}
	}

	return
}

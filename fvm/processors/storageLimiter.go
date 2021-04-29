package processors

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/context"
	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
)

type StorageLimiter struct{}

// TODO update tests to pass env
func (StorageLimiter) Process(
	ctx *context.Context,
	env context.Environment,
	sth *state.StateHolder,
	accounts *state.Accounts,
) error {
	if !ctx.LimitAccountStorage {
		return nil
	}

	addresses := sth.State().UpdatedAddresses()
	for _, address := range addresses {

		// does it exist?
		exists, err := accounts.Exists(address)
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}
		if !exists {
			continue
		}

		capacity, err := env.GetStorageCapacity(common.BytesToAddress(address.Bytes()))
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}

		usage, err := accounts.GetStorageUsed(address)
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}

		if usage > capacity {
			return errors.NewStorageCapacityExceededError(address, usage, capacity)
		}
	}
	return nil
}

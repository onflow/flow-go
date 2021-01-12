package fvm

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/state"
)

type TransactionStorageLimiter struct{}

func NewTransactionStorageLimiter() *TransactionStorageLimiter {
	return &TransactionStorageLimiter{}
}

func (d *TransactionStorageLimiter) Process(
	vm *VirtualMachine,
	ctx Context,
	_ *TransactionProcedure,
	st *state.State,
) error {
	if !ctx.LimitAccountStorage {
		return nil
	}

	env, err := newEnvironment(ctx, vm, st)
	if err != nil {
		return err
	}
	accounts := state.NewAccounts(st)

	addresses := st.UpdatedAddresses()

	for _, address := range addresses {

		// does it exist?
		exists, err := accounts.Exists(address)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		capacity, err := env.GetStorageCapacity(common.BytesToAddress(address.Bytes()))
		if err != nil {
			return err
		}

		usage, err := accounts.GetStorageUsed(address)
		if err != nil {
			return err
		}

		if usage > capacity {
			return &StorageCapacityExceededError{
				Address:         address,
				StorageUsed:     usage,
				StorageCapacity: capacity,
			}
		}
	}
	return nil
}

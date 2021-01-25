package fvm

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/state"
)

type TransactionStorageLimiter struct {
	// A function to create a function to get storage capacity from an address. This is to make this easily testable.
	GetStorageCapacityFuncFactory func(
		vm *VirtualMachine,
		ctx Context,
		tp *TransactionProcedure,
		st *state.State,
	) (func(address common.Address) (value uint64, err error), error)
}

func getStorageCapacityFuncFactory(
	vm *VirtualMachine,
	ctx Context,
	_ *TransactionProcedure,
	st *state.State,
) (func(address common.Address) (value uint64, err error), error) {
	env, err := newEnvironment(ctx, vm, st)
	if err != nil {
		return nil, err
	}
	return func(address common.Address) (value uint64, err error) {
		return env.GetStorageCapacity(common.BytesToAddress(address.Bytes()))
	}, nil
}

func NewTransactionStorageLimiter() *TransactionStorageLimiter {
	return &TransactionStorageLimiter{
		GetStorageCapacityFuncFactory: getStorageCapacityFuncFactory,
	}
}

func (d *TransactionStorageLimiter) Process(
	vm *VirtualMachine,
	ctx Context,
	tp *TransactionProcedure,
	st *state.State,
) error {
	if !ctx.LimitAccountStorage {
		return nil
	}

	getCapacity, err := d.GetStorageCapacityFuncFactory(vm, ctx, tp, st)
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

		capacity, err := getCapacity(common.BytesToAddress(address.Bytes()))
		if err != nil {
			return err
		}

		usage, err := accounts.GetStorageUsed(address)
		if err != nil {
			return err
		}

		if usage > capacity {
			st.Rollback()
			return &StorageCapacityExceededError{
				Address:         address,
				StorageUsed:     usage,
				StorageCapacity: capacity,
			}
		}
	}

	st.Commit()
	return nil
}

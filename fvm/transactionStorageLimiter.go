package fvm

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
)

type TransactionStorageLimiter struct {
	// A function to create a function to get storage capacity from an address. This is to make this easily testable.
	GetStorageCapacityFuncFactory func(
		vm *VirtualMachine,
		ctx Context,
		tp *TransactionProcedure,
		sth *state.StateHolder,
		programs *programs.Programs,
	) (func(address common.Address) (value uint64, err error), error)
	logger zerolog.Logger
}

func getStorageCapacityFuncFactory(
	vm *VirtualMachine,
	ctx Context,
	_ *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) (func(address common.Address) (value uint64, err error), error) {
	env, err := newEnvironment(ctx, vm, sth, programs)
	if err != nil {
		return nil, err
	}
	return func(address common.Address) (value uint64, err error) {
		return env.GetStorageCapacity(common.BytesToAddress(address.Bytes()))
	}, nil
}

func NewTransactionStorageLimiter(logger zerolog.Logger) *TransactionStorageLimiter {
	return &TransactionStorageLimiter{
		GetStorageCapacityFuncFactory: getStorageCapacityFuncFactory,
		logger:                        logger,
	}
}

func (d *TransactionStorageLimiter) Process(
	vm *VirtualMachine,
	ctx *Context,
	tp *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) error {

	if !ctx.LimitAccountStorage {
		return nil
	}

	getCapacity, err := d.GetStorageCapacityFuncFactory(vm, *ctx, tp, sth, programs)
	if err != nil {
		return err
	}
	accounts := state.NewAccounts(sth)

	addresses := sth.State().UpdatedAddresses()

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
			return &StorageCapacityExceededError{
				Address:         address,
				StorageUsed:     usage,
				StorageCapacity: capacity,
			}
		}
	}
	return nil
}

package fvm

import (
	"github.com/onflow/cadence/runtime/common"

	errors "github.com/onflow/flow-go/fvm/errors"
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
}

func getStorageCapacityFuncFactory(
	vm *VirtualMachine,
	ctx Context,
	_ *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) (func(address common.Address) (value uint64, err error), error) {
	env := newEnvironment(ctx, vm, sth, programs)
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
	ctx *Context,
	tp *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) (txError errors.TransactionError, vmError errors.VMError) {
	if !ctx.LimitAccountStorage {
		return nil, nil
	}

	getCapacity, err := d.GetStorageCapacityFuncFactory(vm, *ctx, tp, sth, programs)

	txError, vmError = errors.SplitErrorTypes(err)
	if txError != nil || vmError != nil {
		return
	}

	accounts := state.NewAccounts(sth)

	addresses := sth.State().UpdatedAddresses()

	for _, address := range addresses {

		// does it exist?
		exists, err := accounts.Exists(address)
		if err != nil {
			txError, vmError = errors.SplitErrorTypes(err)
			if txError != nil || vmError != nil {
				return
			}
		}
		if !exists {
			continue
		}

		capacity, err := getCapacity(common.BytesToAddress(address.Bytes()))
		if err != nil {
			txError, vmError = errors.SplitErrorTypes(err)
			if txError != nil || vmError != nil {
				return
			}
		}

		usage, err := accounts.GetStorageUsed(address)
		if err != nil {
			txError, vmError = errors.SplitErrorTypes(err)
			if txError != nil || vmError != nil {
				return
			}
		}

		if usage > capacity {
			return &errors.StorageCapacityExceededError{
				Address:         address,
				StorageUsed:     usage,
				StorageCapacity: capacity,
			}, nil
		}
	}
	return nil, nil
}

package processors

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/context"
	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
)

type StorageLimiter struct {
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
	vm context.VirtualMachine,
	ctx *context.Context,
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
) error {
	if !ctx.LimitAccountStorage {
		return nil
	}

	getCapacity, err := d.GetStorageCapacityFuncFactory(vm, *ctx, tp, sth, programs)

	if err != nil {
		return fmt.Errorf("storage limit check failed: %w", err)
	}

	accounts := state.NewAccounts(sth)

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

		capacity, err := getCapacity(common.BytesToAddress(address.Bytes()))
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

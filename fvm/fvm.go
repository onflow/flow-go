package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

// An Invokable is a procedure that can be executed by the virtual machine.
type Invokable interface {
	Invoke(vm *VirtualMachine, ctx Context, ledger Ledger) error
}

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
	Runtime runtime.Runtime
}

// New creates a new virtual machine instance with the provided runtime.
func New(rt runtime.Runtime) *VirtualMachine {
	return &VirtualMachine{
		Runtime: rt,
	}
}

// Invoke invokes an invokable in a context against the given ledger.
func (vm *VirtualMachine) Invoke(ctx Context, i Invokable, ledger Ledger) error {
	return i.Invoke(vm, ctx, ledger)
}

// GetAccount returns the account with the given address or an error if none exists.
func (vm *VirtualMachine) GetAccount(ctx Context, address flow.Address, ledger Ledger) (*flow.Account, error) {
	account, err := getAccount(vm, ctx, ledger, address)
	if err != nil {
		// TODO: wrap error
		return nil, err
	}

	return account, nil
}

// invokeMetaTransaction invokes a meta transaction inside the context of an outer transaction.
//
// Errors that occur in a meta transaction are propagated as a single error that can be
// captured by the Cadence runtime and eventually disambiguated by the parent context.
func (vm *VirtualMachine) invokeMetaTransaction(ctx Context, tx *InvokableTransaction, ledger Ledger) error {
	ctx = NewContextFromParent(
		ctx,
		WithTransactionProcessors([]TransactionProcessor{
			NewTransactionInvocator(),
		}),
	)

	err := vm.Invoke(ctx, tx, ledger)
	if err != nil {
		return err
	}

	if tx.Err != nil {
		return tx.Err
	}

	return nil
}

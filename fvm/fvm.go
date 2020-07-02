package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

// An Invokable is a procedure that can be executed by the virtual machine.
type Invokable interface {
	Parse(vm *VirtualMachine, ctx Context, ledger Ledger) (Invokable, error)
	Invoke(vm *VirtualMachine, ctx Context, ledger Ledger) (*InvocationResult, error)
}

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
	runtime runtime.Runtime
	chain   flow.Chain
}

// New creates a new virtual machine instance with the provided runtime.
func New(rt runtime.Runtime, chain flow.Chain) *VirtualMachine {
	return &VirtualMachine{
		runtime: rt,
		chain:   chain,
	}
}

// Parse parses an invokable in a context against the given ledger.
func (vm *VirtualMachine) Parse(ctx Context, i Invokable, ledger Ledger) (Invokable, error) {
	return i.Parse(vm, ctx, ledger)
}

// Invoke invokes an invokable in a context against the given ledger.
func (vm *VirtualMachine) Invoke(ctx Context, i Invokable, ledger Ledger) (*InvocationResult, error) {
	return i.Invoke(vm, ctx, ledger)
}

// GetAccount returns the account with the given address or an error if none exists.
func (vm *VirtualMachine) GetAccount(ctx Context, address flow.Address, ledger Ledger) (*flow.Account, error) {
	account, err := getAccount(vm, ctx, ledger, vm.chain, address)
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
func (vm *VirtualMachine) invokeMetaTransaction(ctx Context, tx InvokableTransaction, ledger Ledger) error {
	result, err := vm.Invoke(ctx, tx, ledger)
	if err != nil {
		return err
	}

	if result.Error != nil {
		return result.Error
	}

	return nil
}

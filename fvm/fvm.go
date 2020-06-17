package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

// VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine interface {
	NewContext(opts ...Option) Context
}

type Invokable interface {
	Parse(ctx Context, ledger Ledger) (Invokable, error)
	Invoke(ctx Context, ledger Ledger) (*InvocationResult, error)
}

// New creates a new virtual machine instance with the provided runtime.
func New(rt runtime.Runtime, chain flow.Chain) VirtualMachine {
	return &virtualMachine{rt: rt}
}

type virtualMachine struct {
	rt runtime.Runtime
}

func (vm *virtualMachine) NewContext(opts ...Option) Context {
	return newContext(vm.rt, defaultOptions(), opts...)
}

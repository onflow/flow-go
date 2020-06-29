package fvm

import (
	"github.com/onflow/cadence/runtime"
)

type Invokable interface {
	Parse(ctx Context, ledger Ledger) (Invokable, error)
	Invoke(ctx Context, ledger Ledger) (*InvocationResult, error)
}

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
	rt runtime.Runtime
}

// New creates a new virtual machine instance with the provided runtime.
func New(rt runtime.Runtime) *VirtualMachine {
	return &VirtualMachine{rt: rt}
}

// NewContext initializes a new execution context with the provided options.
func (vm *VirtualMachine) NewContext(opts ...Option) Context {
	return newContext(vm.rt, defaultOptions(), opts...)
}

package fvm

import (
	"github.com/onflow/cadence/runtime"
)

// VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine interface {
	NewContext(opts ...Option) Context
}

type Invokable interface {
	Parse(ctx Context, ledger Ledger) (Invokable, error)
	Invoke(ctx Context, ledger Ledger) (*Result, error)
}

// New creates a new virtual machine instance with the provided runtime.
func New(rt runtime.Runtime) VirtualMachine {
	return &virtualMachine{rt: rt}
}

type virtualMachine struct {
	rt runtime.Runtime
}

func (vm *virtualMachine) NewContext(opts ...Option) Context {
	return newContext(vm.rt, defaultOptions(), opts...)
}

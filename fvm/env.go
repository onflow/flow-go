package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/state"
)

// Environment accepts a context and a virtual machine instance and provides
// cadence runtime interface methods to the runtime.
type Environment interface {
	Context() *Context
	VM() *VirtualMachine
	StateHolder() *state.StateHolder
	runtime.Interface
}

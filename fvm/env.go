package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/handler"
)

// Environment accepts a context and a virtual machine instance and provides
// cadence runtime interface methods to the runtime.
type Environment interface {
	Context() *Context
	VM() *VirtualMachine
	ComputationHandler() *handler.ComputationMeteringHandler
	runtime.Interface
}

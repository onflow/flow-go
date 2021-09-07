package fvm

import (
	"github.com/onflow/cadence/runtime"
)

// Enviornment accepts a context and a virtual machine instance and provides
// cadence runtime interface methods to the runtime.
type Enviornment interface {
	Context() *Context
	VM() *VirtualMachine
	runtime.Interface
}

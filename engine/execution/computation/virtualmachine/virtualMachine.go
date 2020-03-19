package virtualmachine

import (
	"github.com/dapperlabs/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

// VirtualMachine augments the Cadence runtime with the Flow host functionality required
// to execute transactions.
type VirtualMachine interface {
	// NewBlockContext creates a new block context for executing transactions.
	NewBlockContext(b *flow.Header) BlockContext
}

// New creates a new virtual machine instance with the provided runtime.
func New(rt runtime.Runtime) VirtualMachine {
	return &virtualMachine{
		rt: rt,
	}
}

type virtualMachine struct {
	rt runtime.Runtime
}

func (vm *virtualMachine) NewBlockContext(header *flow.Header) BlockContext {
	return &blockContext{
		vm:     vm,
		header: header,
	}
}

func (vm *virtualMachine) executeTransaction(
	script []byte,
	runtimeInterface runtime.Interface,
	location runtime.Location,
) error {
	return vm.rt.ExecuteTransaction(script, runtimeInterface, location)
}

func (vm *virtualMachine) executeScript(
	script []byte,
	runtimeInterface runtime.Interface,
	location runtime.Location,
) (runtime.Value, error) {
	return vm.rt.ExecuteScript(script, runtimeInterface, location)
}

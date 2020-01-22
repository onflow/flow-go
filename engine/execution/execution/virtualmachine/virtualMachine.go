package virtualmachine

import (
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A VirtualMachine augments the Cadence runtime with the Flow host functionality required
// to execute transactions.
type VirtualMachine interface {
	// NewBlockContext creates a new block context for executing transactions.
	NewBlockContext(b *flow.Block) BlockContext
}

// New creates a new virtual machine instance with the provided runtime.
func New(rt runtime.Runtime) VirtualMachine {
	return &virtualMachine{
		rt: rt,
	}
}

type virtualMachine struct {
	rt    runtime.Runtime
	block *flow.Block
}

func (vm *virtualMachine) NewBlockContext(b *flow.Block) BlockContext {
	return &blockContext{
		vm:    vm,
		block: b,
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

package virtualmachine

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	MaxProgramASTCacheSize = 256
)

// VirtualMachine augments the Cadence runtime with the Flow host functionality required
// to execute transactions.
type VirtualMachine interface {
	// NewBlockContext creates a new block context for executing transactions.
	NewBlockContext(b *flow.Header) BlockContext
	// GetCache returns the program AST cache.
	GetCache() ASTCache
}

// New creates a new virtual machine instance with the provided runtime.
func New(rt runtime.Runtime) (VirtualMachine, error) {
	cache, err := NewLRUASTCache(MaxProgramASTCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create vm ast cache, %w", err)
	}
	return &virtualMachine{
		rt:    rt,
		cache: cache,
	}, nil
}

type virtualMachine struct {
	rt    runtime.Runtime
	cache ASTCache
}

func (vm *virtualMachine) NewBlockContext(header *flow.Header) BlockContext {
	return &blockContext{
		vm:     vm,
		header: header,
	}
}

func (vm *virtualMachine) GetCache() ASTCache {
	return vm.cache
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

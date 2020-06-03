package virtualmachine

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	AccountKeyWeightThreshold = 1000
	MaxProgramASTCacheSize    = 256
)

// VirtualMachine augments the Cadence runtime with the Flow host functionality required
// to execute transactions.
type VirtualMachine interface {
	// NewBlockContext creates a new block context for executing transactions.
	NewBlockContext(b *flow.Header, blocks Blocks) BlockContext
	// GetCache returns the program AST cache.
	ASTCache() ASTCache
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

func NewWithCache(rt runtime.Runtime, cache ASTCache) (VirtualMachine, error) {
	return &virtualMachine{
		rt:    rt,
		cache: cache,
	}, nil
}

type virtualMachine struct {
	rt    runtime.Runtime
	cache ASTCache
}

func (vm *virtualMachine) NewBlockContext(header *flow.Header, blocks Blocks) BlockContext {
	return &blockContext{
		vm:     vm,
		header: header,
		blocks: blocks,
	}
}

func (vm *virtualMachine) ASTCache() ASTCache {
	return vm.cache
}

func (vm *virtualMachine) executeTransaction(
	script []byte,
	arguments [][]byte,
	runtimeInterface runtime.Interface,
	location runtime.Location,
) error {
	return vm.rt.ExecuteTransaction(script, arguments, runtimeInterface, location)
}

func (vm *virtualMachine) executeScript(
	script []byte,
	arguments [][]byte,
	runtimeInterface runtime.Interface,
	location runtime.Location,
) (cadence.Value, error) {
	return vm.rt.ExecuteScript(script, arguments, runtimeInterface, location)
}

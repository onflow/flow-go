package virtualmachine

import (
	"encoding/binary"
	"fmt"
	"math/rand"

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
func New(rt runtime.Runtime, options ...VirtualMachineOption) (VirtualMachine, error) {
	vm := &virtualMachine{
		rt: rt,
	}

	for _, option := range options {
		option(vm)
	}

	if vm.cache == nil {
		cache, err := NewLRUASTCache(MaxProgramASTCacheSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create vm ast cache, %w", err)
		}
		vm.cache = cache
	}

	return vm, nil
}

type VirtualMachineOption func(*virtualMachine)

func WithCache(cache ASTCache) VirtualMachineOption {
	return func(ctx *virtualMachine) {
		ctx.cache = cache
	}
}

func WithSimpleAddresses(enabled bool) VirtualMachineOption {
	return func(ctx *virtualMachine) {
		ctx.simpleAddresses = enabled
	}
}

type virtualMachine struct {
	rt              runtime.Runtime
	cache           ASTCache
	simpleAddresses bool
}

func (vm *virtualMachine) NewBlockContext(header *flow.Header, blocks Blocks) BlockContext {
	bc := &blockContext{
		vm:              vm,
		header:          header,
		blocks:          blocks,
		simpleAddresses: vm.simpleAddresses,
	}

	// Seed the random number generator with entropy created from the block header ID. The random number generator will
	// be used by the UnsafeRandom function.
	// TODO: replace with better source of randomness.
	if header != nil {
		id := header.ID()
		bc.rng = rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(id[:]))))
	}

	return bc
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

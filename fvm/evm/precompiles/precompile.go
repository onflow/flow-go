package precompiles

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// Function is an interface for a function in a multi-function precompile contract
type Function interface {
	// FunctionSignature returns the function signature for this function
	FunctionSignature() FunctionSignature

	// ComputeGas computes the gas needed for the given input
	ComputeGas(input []byte) uint64

	// Run runs the function on the given data
	Run(input []byte) ([]byte, error)
}

// MultiFunctionPrecompileContract constructs a multi-function precompile smart contract
func MultiFunctionPrecompileContract(
	address types.Address,
	functions []Function,
) types.Precompile {
	pc := &precompile{
		functions: make(map[FunctionSignature]Function),
		address:   address,
	}
	for _, f := range functions {
		pc.functions[f.FunctionSignature()] = f
	}
	return pc
}

type precompile struct {
	address   types.Address
	functions map[FunctionSignature]Function
}

func (p *precompile) Address() types.Address {
	return p.address
}

// RequiredPrice calculates the contract gas use
func (p *precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	sig, data := SplitFunctionSignature(input)
	callable, found := p.functions[sig]
	if !found {
		return 0
	}
	return callable.ComputeGas(data)
}

// Run runs the precompiled contract
func (p *precompile) Run(input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, fmt.Errorf("invalid method") // TODO return the right error based on geth
	}
	sig, data := SplitFunctionSignature(input)
	callable, found := p.functions[sig]
	if !found {
		return nil, fmt.Errorf("invalid method") // TODO return the right error based on geth
	}
	return callable.Run(data)
}

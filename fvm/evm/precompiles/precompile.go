package precompiles

import (
	"errors"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// InvalidMethodCallGasUsage captures how much gas we charge for invalid method call
const InvalidMethodCallGasUsage = uint64(1)

// ErrInvalidMethodCall is returned when the method is not available on the contract
var ErrInvalidMethodCall = errors.New("invalid method call")

// Function is an interface for a function in a multi-function precompile contract
type Function interface {
	// FunctionSelector returns the function selector bytes for this function
	FunctionSelector() FunctionSelector

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
		functions: make(map[FunctionSelector]Function),
		address:   address,
	}
	for _, f := range functions {
		pc.functions[f.FunctionSelector()] = f
	}
	return pc
}

type precompile struct {
	address   types.Address
	functions map[FunctionSelector]Function
}

func (p *precompile) Address() types.Address {
	return p.address
}

// RequiredGas calculates the contract gas use
func (p *precompile) RequiredGas(input []byte) uint64 {
	if len(input) < FunctionSelectorLength {
		return InvalidMethodCallGasUsage
	}
	sig, data := SplitFunctionSelector(input)
	callable, found := p.functions[sig]
	if !found {
		return InvalidMethodCallGasUsage
	}
	return callable.ComputeGas(data)
}

// Run runs the precompiled contract
func (p *precompile) Run(input []byte) ([]byte, error) {
	if len(input) < FunctionSelectorLength {
		return nil, ErrInvalidMethodCall
	}
	sig, data := SplitFunctionSelector(input)
	callable, found := p.functions[sig]
	if !found {
		return nil, ErrInvalidMethodCall
	}
	return callable.Run(data)
}

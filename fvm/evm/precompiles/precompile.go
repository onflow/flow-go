package precompiles

import (
	"errors"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// TODO from here: add test for captures

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
		functions:        make(map[FunctionSelector]Function),
		address:          address,
		requiredGasCalls: make([]types.RequiredGasCall, 0),
		runCalls:         make([]types.RunCall, 0),
	}
	for _, f := range functions {
		pc.functions[f.FunctionSelector()] = f
	}
	return pc
}

type precompile struct {
	address          types.Address
	functions        map[FunctionSelector]Function
	requiredGasCalls []types.RequiredGasCall
	runCalls         []types.RunCall
}

func (p *precompile) Address() types.Address {
	return p.address
}

// RequiredGas calculates the contract gas use
func (p *precompile) RequiredGas(input []byte) uint64 {
	var output uint64
	defer func() {
		p.requiredGasCalls = append(
			p.requiredGasCalls,
			types.RequiredGasCall{
				Input:  input,
				Output: output,
			})
	}()
	if len(input) < FunctionSelectorLength {
		output = InvalidMethodCallGasUsage
		return output
	}
	sig, data := SplitFunctionSelector(input)
	callable, found := p.functions[sig]
	if !found {
		output = InvalidMethodCallGasUsage
		return output
	}
	output = callable.ComputeGas(data)
	return output
}

// Run runs the precompiled contract
func (p *precompile) Run(input []byte) ([]byte, error) {
	var output []byte
	var err error
	defer func() {
		p.runCalls = append(
			p.runCalls,
			types.RunCall{
				Input:    input,
				Output:   output,
				ErrorMsg: err.Error(),
			})
	}()

	if len(input) < FunctionSelectorLength {
		output = nil
		err = ErrInvalidMethodCall
		return output, err
	}
	sig, data := SplitFunctionSelector(input)
	callable, found := p.functions[sig]
	if !found {
		output = nil
		err = ErrInvalidMethodCall
		return output, err
	}
	output, err = callable.Run(data)
	return output, err
}

func (p *precompile) RequiredGasCalls() []types.RequiredGasCall {
	return p.requiredGasCalls
}

func (p *precompile) RunCalls() []types.RunCall {
	return p.runCalls
}

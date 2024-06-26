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

// MultiFunctionPrecompiledContract constructs a multi-function precompile smart contract
func MultiFunctionPrecompiledContract(
	address types.Address,
	functions []Function,
) types.PrecompiledContract {
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
		// lazy allocation
		if p.requiredGasCalls == nil {
			p.requiredGasCalls = make([]types.RequiredGasCall, 0)
		}
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
func (p *precompile) Run(input []byte) (output []byte, err error) {
	defer func() {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		// lazy allocation
		if p.runCalls == nil {
			p.runCalls = make([]types.RunCall, 0)
		}
		p.runCalls = append(
			p.runCalls,
			types.RunCall{
				Input:    input,
				Output:   output,
				ErrorMsg: errMsg,
			})
	}()

	if len(input) < FunctionSelectorLength {
		output = nil
		err = ErrInvalidMethodCall
		return
	}
	sig, data := SplitFunctionSelector(input)
	callable, found := p.functions[sig]
	if !found {
		output = nil
		err = ErrInvalidMethodCall
		return
	}
	output, err = callable.Run(data)
	return
}

func (p *precompile) CapturedCalls() *types.PrecompiledCalls {
	return &types.PrecompiledCalls{
		Address:          p.address,
		RequiredGasCalls: p.requiredGasCalls,
		RunCalls:         p.runCalls,
	}
}

func (p *precompile) Reset() {
	p.requiredGasCalls = nil
	p.runCalls = nil
}

package precompiles

import (
	"fmt"
	"strings"

	gethCrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// This is derived as the first 4 bytes of the Keccak hash of the ASCII form of the signature of the method
type FunctionSignature [4]byte

// ComputeFunctionSignature computes the function signture
// given the canonical name of function and args.
// for example the canonical format for int is int256
func ComputeFunctionSignature(name string, args []string) FunctionSignature {
	var sig FunctionSignature
	input := fmt.Sprintf("%v(%v)", name, strings.Join(args, ","))
	copy(sig[0:4], gethCrypto.Keccak256([]byte(input))[:4])
	return sig
}

type Function interface {
	FunctionSignature() FunctionSignature

	ComputeGas(input []byte) uint64

	Run(input []byte) ([]byte, error)
}

func multiMethodPrecompileContract(
	address types.Address,
	functions map[FunctionSignature]Function,
) types.Precompile {
	return &Precompile{
		functions: functions,
		address:   address,
	}
}

type Precompile struct {
	functions map[FunctionSignature]Function
	address   types.Address
}

func (p *Precompile) Address() types.Address {
	return p.address
}

// RequiredPrice calculates the contract gas use
func (p *Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	sig, data := splitFunctionSignature(input)
	callable, found := p.functions[sig]
	if !found {
		return 0
	}
	return callable.ComputeGas(data)
}

// Run runs the precompiled contract
func (p *Precompile) Run(input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, fmt.Errorf("invalid method") // TODO return the right error based on geth
	}
	sig, data := splitFunctionSignature(input)
	callable, found := p.functions[sig]
	if !found {
		return nil, fmt.Errorf("invalid method") // TODO return the right error based on geth
	}
	return callable.Run(data)
}

func splitFunctionSignature(input []byte) (FunctionSignature, []byte) {
	var funcSig FunctionSignature
	copy(funcSig[:], input[0:4])
	return funcSig, input[4:]
}

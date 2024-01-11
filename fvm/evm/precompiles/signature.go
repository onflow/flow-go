package precompiles

import (
	"fmt"
	"strings"

	gethCrypto "github.com/ethereum/go-ethereum/crypto"
)

// This is derived as the first 4 bytes of the Keccak hash of the ASCII form of the signature of the method
type FunctionSignature [4]byte

func (fs FunctionSignature) Bytes() []byte {
	return fs[:]
}

// ComputeFunctionSignature computes the function signture
// given the canonical name of function and args.
// for example the canonical format for int is int256
func ComputeFunctionSignature(name string, args []string) FunctionSignature {
	var sig FunctionSignature
	input := fmt.Sprintf("%v(%v)", name, strings.Join(args, ","))
	copy(sig[0:4], gethCrypto.Keccak256([]byte(input))[:4])
	return sig
}

// SplitFunctionSignature splits the function signature from input data and
// returns the rest of the data
func SplitFunctionSignature(input []byte) (FunctionSignature, []byte) {
	var funcSig FunctionSignature
	copy(funcSig[:], input[0:4])
	return funcSig, input[4:]
}

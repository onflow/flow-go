package precompiles

import (
	"fmt"
	"strings"

	gethCrypto "github.com/ethereum/go-ethereum/crypto"
)

const FunctionSelectorLength = 4

// This is derived as the first 4 bytes of the Keccak hash of the ASCII form of the signature of the method
type FunctionSelector [FunctionSelectorLength]byte

func (fs FunctionSelector) Bytes() []byte {
	return fs[:]
}

// ComputeFunctionSelector computes the function selector
// given the canonical name of function and args.
// for example the canonical format for int is int256
func ComputeFunctionSelector(name string, args []string) FunctionSelector {
	var sig FunctionSelector
	input := fmt.Sprintf("%v(%v)", name, strings.Join(args, ","))
	copy(sig[0:FunctionSelectorLength], gethCrypto.Keccak256([]byte(input))[:FunctionSelectorLength])
	return sig
}

// SplitFunctionSelector splits the function signature from input data and
// returns the rest of the data
func SplitFunctionSelector(input []byte) (FunctionSelector, []byte) {
	var funcSig FunctionSelector
	copy(funcSig[:], input[0:FunctionSelectorLength])
	return funcSig, input[FunctionSelectorLength:]
}

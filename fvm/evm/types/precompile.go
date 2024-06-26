package types

import (
	gethVM "github.com/onflow/go-ethereum/core/vm"
	"github.com/onflow/go-ethereum/rlp"
)

// Precompile wraps gethVM precompiles with
// functionality to hold on to the deployed address
// and captures calls to its method.
type Precompile interface {
	gethVM.PrecompiledContract
	// returns the address where the precompile is deployed
	Address() Address
	// CapturedCalls returns a list of calls to the Run and RequiredGas
	CapturedCalls() *PrecompiledCalls
	// Reset resets the list of captured calls
	Reset()
}

// RunCall captures a call to the RequiredGas method of a precompile contract
type RequiredGasCall struct {
	Input  []byte
	Output uint64
}

// RunCall captures a call to the Run method of a precompile contract
type RunCall struct {
	Input    []byte
	Output   []byte
	ErrorMsg string
}

// PrecompiledCalls captures all the called to a precompiled contract
type PrecompiledCalls struct {
	Address          Address
	RequiredGasCalls []RequiredGasCall
	RunCalls         []RunCall
}

func (pc *PrecompiledCalls) isEmpty() bool {
	return len(pc.RequiredGasCalls) == 0 && len(pc.RunCalls) == 0
}

type AggregatedPrecompiledCalls []PrecompiledCalls

func (apc AggregatedPrecompiledCalls) isEmpty() bool {
	isEmpty := true
	for _, ap := range apc {
		if !ap.isEmpty() {
			isEmpty = false
		}
	}
	return isEmpty
}

// Encode encodes the a precompile calls type using rlp encoding
func (apc AggregatedPrecompiledCalls) Encode() ([]byte, error) {
	// optimization for empty case which would be most of transactions
	if apc.isEmpty() {
		return []byte{}, nil
	}
	return rlp.EncodeToBytes(apc)
}

// AggregatedPrecompileCallsFromEncoded constructs an AggregatedPrecompileCalls from encoded data
func AggregatedPrecompileCallsFromEncoded(encoded []byte) (AggregatedPrecompiledCalls, error) {
	apc := make([]PrecompiledCalls, 0)
	if len(encoded) == 0 {
		return apc, nil
	}
	return apc, rlp.DecodeBytes(encoded, apc)
}

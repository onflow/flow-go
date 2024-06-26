package types

import (
	gethVM "github.com/onflow/go-ethereum/core/vm"
	"github.com/onflow/go-ethereum/rlp"
)

// PrecompiledContract wraps gethVM precompiles with
// functionality to hold on to the deployed address
// and captures calls to its method.
type PrecompiledContract interface {
	// PrecompiledContract provides an interface for
	// calling requiredGas and run
	gethVM.PrecompiledContract
	// Address returns the address where the precompile is deployed
	Address() Address
	// IsCalled returned true if any of the methods on this precompiled is called
	IsCalled() bool
	// CapturedCalls returns a list of calls to the Run and RequiredGas methods
	// it includes the input and returned value for each call
	CapturedCalls() *PrecompiledCalls
	// Reset resets the list of captured calls
	Reset()
}

// RunCall captures a call to the RequiredGas method of a precompiled contract
type RequiredGasCall struct {
	Input  []byte
	Output uint64
}

// RunCall captures a call to the Run method of a precompiled contract
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

// IsEmpty returns true if no requiredGas or run calls is captured
func (pc *PrecompiledCalls) IsEmpty() bool {
	return len(pc.RequiredGasCalls) == 0 && len(pc.RunCalls) == 0
}

// AggregatedPrecompiledCalls aggregates a list of precompiled calls
type AggregatedPrecompiledCalls []PrecompiledCalls

// IsEmpty returns true if all of the underlying precompiled calls are empty
func (apc AggregatedPrecompiledCalls) IsEmpty() bool {
	isEmpty := true
	for _, ap := range apc {
		if !ap.IsEmpty() {
			isEmpty = false
		}
	}
	return isEmpty
}

// Encode encodes the a precompile calls type using rlp encoding
// if there is no underlying call, we encode to empty bytes to save
// space on transaction results (common case)
func (apc AggregatedPrecompiledCalls) Encode() ([]byte, error) {
	// optimization for empty case which would be most of transactions
	if apc.IsEmpty() {
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
	return apc, rlp.DecodeBytes(encoded, &apc)
}

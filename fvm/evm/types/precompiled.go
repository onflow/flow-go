package types

import (
	"bytes"

	gethVM "github.com/onflow/go-ethereum/core/vm"
	"github.com/onflow/go-ethereum/rlp"
)

// PrecompiledContract wraps gethVM precompiles with
// functionality to return where the contract is deployed
type PrecompiledContract interface {
	// PrecompiledContract provides an interface for
	// calling requiredGas and run
	gethVM.PrecompiledContract
	// Address returns the address where the precompile is deployed
	Address() Address
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

// PrecompiledCalls captures all the calls to a precompiled contract
type PrecompiledCalls struct {
	Address          Address
	RequiredGasCalls []RequiredGasCall
	RunCalls         []RunCall
}

// IsEmpty returns true if no requiredGas or run calls is captured
func (pc *PrecompiledCalls) IsEmpty() bool {
	return len(pc.RequiredGasCalls) == 0 && len(pc.RunCalls) == 0
}

const (
	AggregatedPrecompiledCallsEncodingVersion  uint8 = 1
	AggregatedPrecompiledCallsEncodingByteSize int   = 1
)

// AggregatedPrecompiledCalls aggregates a list of precompiled calls
// the list should be sorted by the address
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

// Encode encodes the aggregated precompile calls using rlp encoding
// if there is no underlying call, we encode to empty bytes to save
// space on transaction results (common case)
func (apc AggregatedPrecompiledCalls) Encode() ([]byte, error) {
	if apc.IsEmpty() {
		return []byte{}, nil
	}
	buffer := bytes.NewBuffer(make([]byte, 0))
	// write the encoding version
	buffer.WriteByte(AggregatedPrecompiledCallsEncodingVersion)
	// then RLP encode
	err := rlp.Encode(buffer, apc)
	return buffer.Bytes(), err
}

// AggregatedPrecompileCallsFromEncoded constructs an AggregatedPrecompileCalls from encoded data
func AggregatedPrecompileCallsFromEncoded(encoded []byte) (AggregatedPrecompiledCalls, error) {
	apc := make([]PrecompiledCalls, 0)
	if len(encoded) == 0 {
		return apc, nil
	}
	return apc, rlp.DecodeBytes(encoded[AggregatedPrecompiledCallsEncodingByteSize:], &apc)
}

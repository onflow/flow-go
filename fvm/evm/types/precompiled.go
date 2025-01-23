package types

import (
	"bytes"
	"fmt"

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

// RunCall captures a call to the Run method of a precompiled contract
type RunCall struct {
	Output   []byte
	ErrorMsg string
}

// PrecompiledCalls captures all the calls to a precompiled contract
type PrecompiledCalls struct {
	Address          Address
	RequiredGasCalls []uint64
	RunCalls         []RunCall
}

// IsEmpty returns true if no requiredGas or run calls is captured
func (pc *PrecompiledCalls) IsEmpty() bool {
	return len(pc.RequiredGasCalls) == 0 && len(pc.RunCalls) == 0
}

const (
	AggregatedPrecompiledCallsEncodingByteSize int   = 1
	AggregatedPrecompiledCallsEncodingVersion  uint8 = 2 // current version
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
// TODO: In the future versions of the encoding we might skip encoding the inputs
// given it just takes space and not needed during execution time
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
	switch int(encoded[0]) {
	case 1:
		return decodePrecompiledCallsV1(encoded)
	case 2:
		return apc, rlp.DecodeBytes(encoded[AggregatedPrecompiledCallsEncodingByteSize:], &apc)
	default:
		return nil, fmt.Errorf("unknown type for encoded AggregatedPrecompiledCalls received %d", int(encoded[0]))
	}
}

func decodePrecompiledCallsV1(encoded []byte) (AggregatedPrecompiledCalls, error) {
	legacy := make([]precompiledCallsV1, 0)
	err := rlp.DecodeBytes(encoded[AggregatedPrecompiledCallsEncodingByteSize:], &legacy)
	if err != nil {
		return nil, err
	}
	apc := make([]PrecompiledCalls, len(legacy))
	for i, ap := range legacy {
		reqCalls := make([]uint64, len(ap.RequiredGasCalls))
		for j, rc := range ap.RequiredGasCalls {
			reqCalls[j] = rc.Output
		}
		runCalls := make([]RunCall, len(ap.RunCalls))
		for j, rc := range ap.RunCalls {
			runCalls[j] = RunCall{
				Output:   rc.Output,
				ErrorMsg: rc.ErrorMsg,
			}
		}
		apc[i] = PrecompiledCalls{
			Address:          ap.Address,
			RequiredGasCalls: reqCalls,
			RunCalls:         runCalls,
		}
	}
	return apc, nil
}

// legacy encoding types
type precompiledCallsV1 struct {
	Address          Address
	RequiredGasCalls []struct {
		Input  []byte
		Output uint64
	}
	RunCalls []struct {
		Input    []byte
		Output   []byte
		ErrorMsg string
	}
}

package emulator

import (
	"bytes"
	"sort"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// CallTracker captures precompiled calls
type CallTracker struct {
	callsByAddress map[types.Address]*types.PrecompiledCalls
}

// NewCallTracker  constructs a new CallTracker
func NewCallTracker() *CallTracker {
	return &CallTracker{}
}

// RegisterPrecompiledContract registers a precompiled contract for tracking
func (ct *CallTracker) RegisterPrecompiledContract(pc types.PrecompiledContract) types.PrecompiledContract {
	return &WrappedPrecompiledContract{
		pc: pc,
		ct: ct,
	}
}

// CaptureRequiredGas captures a required gas call
func (ct *CallTracker) CaptureRequiredGas(address types.Address, input []byte, output uint64) {
	if ct.callsByAddress == nil {
		ct.callsByAddress = make(map[types.Address]*types.PrecompiledCalls)
	}
	calls, found := ct.callsByAddress[address]
	if !found {
		calls = &types.PrecompiledCalls{
			Address: address,
		}
		ct.callsByAddress[address] = calls
	}

	calls.RequiredGasCalls = append(calls.RequiredGasCalls, types.RequiredGasCall{
		Input:  input,
		Output: output,
	})
}

// CaptureRun captures a run calls
func (ct *CallTracker) CaptureRun(address types.Address, input []byte, output []byte, err error) {
	if ct.callsByAddress == nil {
		ct.callsByAddress = make(map[types.Address]*types.PrecompiledCalls)
	}
	calls, found := ct.callsByAddress[address]
	if !found {
		calls = &types.PrecompiledCalls{
			Address: address,
		}
		ct.callsByAddress[address] = calls
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	calls.RunCalls = append(calls.RunCalls, types.RunCall{
		Input:    input,
		Output:   output,
		ErrorMsg: errMsg,
	})
}

// IsCalled returns true if any calls has been captured
func (ct *CallTracker) IsCalled() bool {
	return len(ct.callsByAddress) != 0
}

// Encoded
func (ct *CallTracker) CapturedCalls() ([]byte, error) {
	if !ct.IsCalled() {
		return nil, nil
	}
	// else constructs an aggregated precompiled calls
	apc := make(types.AggregatedPrecompiledCalls, 0)

	sortedAddresses := make([]types.Address, 0, len(ct.callsByAddress))
	// we need to sort by address to stay deterministic
	for addr := range ct.callsByAddress {
		sortedAddresses = append(sortedAddresses, addr)
	}

	sort.Slice(sortedAddresses,
		func(i, j int) bool {
			return bytes.Compare(sortedAddresses[i][:], sortedAddresses[j][:]) < 0
		})

	for _, addr := range sortedAddresses {
		apc = append(apc, *ct.callsByAddress[addr])
	}

	return apc.Encode()
}

// Resets the tracker
func (ct *CallTracker) Reset() {
	ct.callsByAddress = nil
}

type WrappedPrecompiledContract struct {
	pc types.PrecompiledContract
	ct *CallTracker
}

func (wpc *WrappedPrecompiledContract) Address() types.Address {
	return wpc.pc.Address()
}
func (wpc *WrappedPrecompiledContract) RequiredGas(input []byte) uint64 {
	output := wpc.pc.RequiredGas(input)
	wpc.ct.CaptureRequiredGas(wpc.pc.Address(), input, output)
	return output
}

func (wpc *WrappedPrecompiledContract) Run(input []byte) ([]byte, error) {
	output, err := wpc.pc.Run(input)
	wpc.ct.CaptureRun(wpc.pc.Address(), input, output, err)
	return output, err
}

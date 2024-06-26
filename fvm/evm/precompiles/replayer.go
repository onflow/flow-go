package precompiles

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/fvm/evm/types"
)

var (
	// errInvalidPrecompiledContractCalls is returned when an invalid list of precompiled contract calls
	// is passed
	errInvalidPrecompiledContractCalls = fmt.Errorf("invalid list of precompiled contract calls")
	// errUnexpectedCall is returned when a call to the precompile is not expected
	errUnexpectedCall = fmt.Errorf("unexpected call")
	// this error is returned where more calls were expected before CapturedCalls call
	errMoreCallsWereExpected = fmt.Errorf("expecting more call")
)

// converts an aggregated set of precompile calls
// into a list of replayer precompiled contract
func AggregatedPrecompiledCallsToPrecompiledContracts(apc types.AggregatedPrecompiledCalls) []types.PrecompiledContract {
	res := make([]types.PrecompiledContract, 0)
	for _, ap := range apc {
		res = append(res, NewReplayerPrecompiledContract(&ap))
	}
	return res
}

// ReplayerPrecompiledContract is a precompiled contract
// that replay the outputs based on the input
type ReplayerPrecompiledContract struct {
	expectedCalls              *types.PrecompiledCalls
	requiredGasIndex, runIndex int
}

// NewReplayerPrecompiledContract constructs a ReplayerPrecompiledContract
func NewReplayerPrecompiledContract(
	expectedCalls *types.PrecompiledCalls,
) *ReplayerPrecompiledContract {
	if expectedCalls == nil {
		panic(errInvalidPrecompiledContractCalls)
	}
	return &ReplayerPrecompiledContract{
		expectedCalls: expectedCalls,
	}
}

func (p *ReplayerPrecompiledContract) Address() types.Address {
	return p.expectedCalls.Address
}

func (p *ReplayerPrecompiledContract) RequiredGas(input []byte) (output uint64) {
	if p.requiredGasIndex > len(p.expectedCalls.RequiredGasCalls) {
		panic(errUnexpectedCall)
	}
	if !bytes.Equal(p.expectedCalls.RequiredGasCalls[p.requiredGasIndex].Input, input) {
		panic(errUnexpectedCall)
	}
	output = p.expectedCalls.RequiredGasCalls[p.requiredGasIndex].Output
	p.requiredGasIndex++
	return
}

func (p *ReplayerPrecompiledContract) Run(input []byte) (output []byte, err error) {
	if p.runIndex > len(p.expectedCalls.RunCalls) {
		panic(errUnexpectedCall)
	}
	if !bytes.Equal(p.expectedCalls.RunCalls[p.runIndex].Input, input) {
		panic(errUnexpectedCall)
	}
	output = p.expectedCalls.RunCalls[p.runIndex].Output
	errMsg := p.expectedCalls.RunCalls[p.runIndex].ErrorMsg
	if len(errMsg) > 0 {
		err = errors.New(errMsg)
	}
	p.runIndex++
	return
}

func (p *ReplayerPrecompiledContract) IsCalled() bool {
	return p.requiredGasIndex > 0 || p.runIndex > 0
}

func (p *ReplayerPrecompiledContract) HasReplayedAll() bool {
	return len(p.expectedCalls.RequiredGasCalls) == p.requiredGasIndex &&
		len(p.expectedCalls.RunCalls) == p.runIndex
}

func (p *ReplayerPrecompiledContract) CapturedCalls() *types.PrecompiledCalls {
	// we didn't consume all calls
	if !p.HasReplayedAll() {
		panic(errMoreCallsWereExpected)
	}
	return p.expectedCalls
}

func (p *ReplayerPrecompiledContract) Reset() {
	p.requiredGasIndex = 0
	p.runIndex = 0
}

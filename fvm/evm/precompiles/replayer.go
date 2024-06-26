package precompiles

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/fvm/evm/types"
)

var (
	errInvalidPrecompiledContractCalls = fmt.Errorf("invalid list of precompiled contract calls")
	// errUnexpectedCall is returned when a call to the precompile is not expected
	errUnexpectedCall = fmt.Errorf("unexpected call")
	// more calls were expected
	errMoreCallsWereExpected = fmt.Errorf("expecting more call")
)

// ReplayerPrecompiledContract is a precompiled contract
// that replay the outputs based on the input
type ReplayerPrecompiledContract struct {
	expectedCalls              *types.PrecompiledCalls
	requiredGasIndex, runIndex int
}

// NewReplayerPrecompiledContract constructs a ReplayerPrecompiledContract
func NewReplayerPrecompiledContract(
	expectedCalls *types.PrecompiledCalls,
) types.PrecompiledContract {
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
	p.requiredGasIndex++
	return p.expectedCalls.RequiredGasCalls[p.requiredGasIndex].Output
}

func (p *ReplayerPrecompiledContract) Run(input []byte) (output []byte, err error) {
	if p.runIndex > len(p.expectedCalls.RunCalls) {
		panic(errUnexpectedCall)
	}
	if !bytes.Equal(p.expectedCalls.RunCalls[p.requiredGasIndex].Input, input) {
		panic(errUnexpectedCall)
	}
	p.runIndex++
	output = p.expectedCalls.RunCalls[p.runIndex].Output
	errMsg := p.expectedCalls.RunCalls[p.runIndex].ErrorMsg
	if len(errMsg) > 0 {
		err = errors.New(errMsg)
	}
	return
}

func (p *ReplayerPrecompiledContract) IsCalled() bool {
	return p.requiredGasIndex > 0 || p.runIndex > 0
}

func (p *ReplayerPrecompiledContract) CapturedCalls() *types.PrecompiledCalls {
	// we didn't consume all calls
	if len(p.expectedCalls.RequiredGasCalls) != p.requiredGasIndex ||
		len(p.expectedCalls.RunCalls) != p.runIndex {
		panic(errMoreCallsWereExpected)
	}
	return p.expectedCalls
}

func (p *ReplayerPrecompiledContract) Reset() {
	p.requiredGasIndex = 0
	p.runIndex = 0
}

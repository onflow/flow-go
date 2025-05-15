package emulator_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
)

func TestTracker(t *testing.T) {
	apc := testutils.AggregatedPrecompiledCallsFixture(t)
	var runCallCounter int
	var requiredGasCallCounter int

	reqGasCallInputs := make([][]byte, len(apc[0].RequiredGasCalls))
	runCallInputs := make([][]byte, len(apc[0].RunCalls))

	for i := range apc[0].RequiredGasCalls {
		reqGasCallInputs[i] = testutils.RandomData(t)
	}

	for i := range apc[0].RunCalls {
		runCallInputs[i] = testutils.RandomData(t)
	}

	pc := &MockedPrecompiled{
		AddressFunc: func() types.Address {
			return apc[0].Address
		},
		RequiredGasFunc: func(input []byte) uint64 {
			res := apc[0].RequiredGasCalls[requiredGasCallCounter]
			require.Equal(t, reqGasCallInputs[requiredGasCallCounter], input)
			requiredGasCallCounter += 1
			return res
		},
		RunFunc: func(input []byte) ([]byte, error) {
			res := apc[0].RunCalls[runCallCounter]
			require.Equal(t, runCallInputs[runCallCounter], input)
			runCallCounter += 1
			var err error
			if len(res.ErrorMsg) > 0 {
				err = errors.New(res.ErrorMsg)
			}
			return res.Output, err
		},
	}
	tracker := emulator.NewCallTracker()
	wpc := tracker.RegisterPrecompiledContract(pc)

	require.Equal(t, apc[0].Address, wpc.Address())

	for _, pc := range apc {
		for i, call := range pc.RequiredGasCalls {
			require.Equal(t, call, wpc.RequiredGas(reqGasCallInputs[i]))
		}
		for i, call := range pc.RunCalls {
			ret, err := wpc.Run(runCallInputs[i])
			require.Equal(t, call.Output, ret)
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			require.Equal(t, call.ErrorMsg, errMsg)
		}

	}
	require.True(t, tracker.IsCalled())

	expectedEncoded, err := apc.Encode()
	require.NoError(t, err)
	encoded, err := tracker.CapturedCalls()
	require.NoError(t, err)
	require.Equal(t, expectedEncoded, encoded)
}

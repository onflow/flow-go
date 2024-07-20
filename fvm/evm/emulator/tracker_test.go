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
	pc := &MockedPrecompiled{
		AddressFunc: func() types.Address {
			return apc[0].Address
		},
		RequiredGasFunc: func(input []byte) uint64 {
			res := apc[0].RequiredGasCalls[requiredGasCallCounter]
			require.Equal(t, res.Input, input)
			requiredGasCallCounter += 1
			return res.Output
		},
		RunFunc: func(input []byte) ([]byte, error) {
			res := apc[0].RunCalls[runCallCounter]
			require.Equal(t, res.Input, input)
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
		for _, call := range pc.RequiredGasCalls {
			require.Equal(t, call.Output, wpc.RequiredGas(call.Input))
		}
		for _, call := range pc.RunCalls {
			ret, err := wpc.Run(call.Input)
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

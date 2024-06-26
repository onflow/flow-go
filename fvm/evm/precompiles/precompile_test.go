package precompiles_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/testutils"
)

func TestMutiFunctionContract(t *testing.T) {
	t.Parallel()

	address := testutils.RandomAddress(t)
	sig := precompiles.FunctionSelector{1, 2, 3, 4}
	data := "data"
	input := append(sig[:], data...)
	gas := uint64(20)
	output := []byte("output")

	pc := precompiles.MultiFunctionPrecompiledContract(address, []precompiles.Function{
		&mockedFunction{
			FunctionSelectorFunc: func() precompiles.FunctionSelector {
				return sig
			},
			ComputeGasFunc: func(inp []byte) uint64 {
				require.Equal(t, []byte(data), inp)
				return gas
			},
			RunFunc: func(inp []byte) ([]byte, error) {
				require.Equal(t, []byte(data), inp)
				return output, nil
			},
		}})

	require.False(t, pc.IsCalled())
	require.Equal(t, address, pc.Address())
	require.Equal(t, gas, pc.RequiredGas(input))
	require.True(t, pc.IsCalled())
	ret, err := pc.Run(input)
	require.NoError(t, err)
	require.Equal(t, output, ret)

	input2 := []byte("non existing signature and data")
	_, err = pc.Run(input2)
	require.Equal(t, precompiles.ErrInvalidMethodCall, err)

	calls := pc.CapturedCalls()
	require.Equal(t, address, calls.Address)
	require.Len(t, calls.RequiredGasCalls, 1)
	require.Equal(t, input, calls.RequiredGasCalls[0].Input)
	require.Equal(t, gas, calls.RequiredGasCalls[0].Output)
	require.Len(t, calls.RunCalls, 2)
	require.Equal(t, input, calls.RunCalls[0].Input)
	require.Equal(t, output, calls.RunCalls[0].Output)
	require.Empty(t, calls.RunCalls[0].ErrorMsg)
	require.Equal(t, input2, calls.RunCalls[1].Input)
	require.Empty(t, calls.RunCalls[1].Output)
	require.Equal(t, precompiles.ErrInvalidMethodCall.Error(), calls.RunCalls[1].ErrorMsg)

	// test reset functionality
	pc.Reset()
	require.False(t, pc.IsCalled())

	calls = pc.CapturedCalls()
	require.Equal(t, address, calls.Address)
	require.Len(t, calls.RequiredGasCalls, 0)
	require.Len(t, calls.RunCalls, 0)
}

type mockedFunction struct {
	FunctionSelectorFunc func() precompiles.FunctionSelector
	ComputeGasFunc       func(input []byte) uint64
	RunFunc              func(input []byte) ([]byte, error)
}

func (mf *mockedFunction) FunctionSelector() precompiles.FunctionSelector {
	if mf.FunctionSelectorFunc == nil {
		panic("method not set for mocked function")
	}
	return mf.FunctionSelectorFunc()
}

func (mf *mockedFunction) ComputeGas(input []byte) uint64 {
	if mf.ComputeGasFunc == nil {
		panic("method not set for mocked function")
	}
	return mf.ComputeGasFunc(input)
}

func (mf *mockedFunction) Run(input []byte) ([]byte, error) {
	if mf.RunFunc == nil {
		panic("method not set for mocked function")
	}
	return mf.RunFunc(input)
}

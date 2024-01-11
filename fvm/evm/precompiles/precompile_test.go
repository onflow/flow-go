package precompiles_test

import (
	"testing"

	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/stretchr/testify/require"
)

func TestMutiFunctionContract(t *testing.T) {
	address := testutils.RandomAddress(t)
	sig := precompiles.FunctionSignature{1, 2, 3, 4}
	data := "data"
	input := append(sig[:], data...)
	gas := uint64(20)
	output := []byte("output")

	pc := precompiles.MultiFunctionPrecompileContract(address, []precompiles.Function{
		&mockedFunction{
			FunctionSignatureFunc: func() precompiles.FunctionSignature {
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

	require.Equal(t, address, pc.Address())
	require.Equal(t, gas, pc.RequiredGas(input))
	ret, err := pc.Run(input)
	require.NoError(t, err)
	require.Equal(t, output, ret)

	input2 := []byte("non existing signature and data")
	_, err = pc.Run(input2)
	require.Error(t, err)
}

type mockedFunction struct {
	FunctionSignatureFunc func() precompiles.FunctionSignature
	ComputeGasFunc        func(input []byte) uint64
	RunFunc               func(input []byte) ([]byte, error)
}

func (mf *mockedFunction) FunctionSignature() precompiles.FunctionSignature {
	if mf.FunctionSignatureFunc == nil {
		panic("method not set for mocked function")
	}
	return mf.FunctionSignatureFunc()
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

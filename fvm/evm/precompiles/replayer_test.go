package precompiles_test

import (
	"testing"

	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayer(t *testing.T) {

	address := testutils.RandomAddress(t)
	input1 := []byte{0, 1}
	input2 := []byte{2, 3}
	gas1 := uint64(1)
	gas2 := uint64(2)
	output1 := []byte{4, 5}
	output2 := []byte{}
	errMsg2 := "some error message"

	pc := &types.PrecompiledCalls{
		Address: address,
		RequiredGasCalls: []types.RequiredGasCall{
			{
				Input:  input1,
				Output: gas1,
			},
			{
				Input:  input2,
				Output: gas2,
			},
		},
		RunCalls: []types.RunCall{
			{
				Input:  input1,
				Output: output1,
			},
			{
				Input:    input2,
				Output:   output2,
				ErrorMsg: errMsg2,
			},
		},
	}

	rep := precompiles.NewReplayerPrecompiledContract(pc)
	require.Equal(t, address, rep.Address())
	require.False(t, rep.HasReplayedAll())

	require.Equal(t, gas1, rep.RequiredGas(input1))
	ret, err := rep.Run(input1)
	require.NoError(t, err)
	require.Equal(t, output1, ret)
	require.False(t, rep.HasReplayedAll())

	require.Equal(t, gas2, rep.RequiredGas(input2))
	ret, err = rep.Run(input2)
	require.Equal(t, errMsg2, err.Error())
	require.Equal(t, output2, ret)

	require.True(t, rep.HasReplayedAll())

	assert.Panics(t, func() { _ = rep.RequiredGas(input2) }, "expected to panic")
	assert.Panics(t, func() { _, _ = rep.Run(input2) }, "expected to panic")
}

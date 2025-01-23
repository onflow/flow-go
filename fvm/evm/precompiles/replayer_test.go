package precompiles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
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
		RequiredGasCalls: []uint64{
			gas1,
			gas2,
		},
		RunCalls: []types.RunCall{
			{
				Output: output1,
			},
			{
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

package precompiles_test

import (
	"testing"

	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/stretchr/testify/require"
)

func TestArchContract(t *testing.T) {
	address := testutils.RandomAddress(t)

	height := uint64(12)
	pc := precompiles.ArchContract(
		address,
		func() (uint64, error) {
			return height, nil
		},
	)

	input := precompiles.FlowBlockHeightFuncSig.Bytes()
	require.Equal(t, address, pc.Address())
	require.Equal(t, precompiles.FlowBlockHeightFixedGas, pc.RequiredGas(input))
	ret, err := pc.Run(input)
	require.NoError(t, err)

	expected := make([]byte, 32)
	expected[31] = 12
	require.Equal(t, expected, ret)

	_, err = pc.Run([]byte{1, 2, 3})
	require.Error(t, err)
}

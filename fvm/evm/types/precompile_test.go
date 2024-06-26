package types_test

import (
	"testing"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/stretchr/testify/require"
)

func TestPrecompiledCallsEncoding(t *testing.T) {

	// empty precompiled calls
	empty := types.AggregatedPrecompiledCalls{
		types.PrecompiledCalls{
			Address: testutils.RandomAddress(t),
		},
		types.PrecompiledCalls{
			Address: testutils.RandomAddress(t),
		},
	}

	encoded, err := empty.Encode()
	require.NoError(t, err)
	require.Empty(t, encoded)

	apc := types.AggregatedPrecompiledCalls{
		types.PrecompiledCalls{
			Address: testutils.RandomAddress(t),
			RequiredGasCalls: []types.RequiredGasCall{{
				Input:  []byte{1},
				Output: 2,
			}},
			RunCalls: []types.RunCall{},
		},
	}

	encoded, err = apc.Encode()
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	ret, err := types.AggregatedPrecompileCallsFromEncoded(encoded)
	require.NoError(t, err)
	require.Equal(t, apc, ret)

	apc = types.AggregatedPrecompiledCalls{
		types.PrecompiledCalls{
			Address: testutils.RandomAddress(t),
			RequiredGasCalls: []types.RequiredGasCall{{
				Input:  []byte{1},
				Output: 2,
			}},
			RunCalls: []types.RunCall{
				{
					Input:  []byte{3, 4},
					Output: []byte{5, 6},
				},
				{
					Input:    []byte{3, 4},
					Output:   []byte{},
					ErrorMsg: "Some error msg",
				},
			},
		},
	}

	encoded, err = apc.Encode()
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	ret, err = types.AggregatedPrecompileCallsFromEncoded(encoded)
	require.NoError(t, err)
	require.Equal(t, apc, ret)
}

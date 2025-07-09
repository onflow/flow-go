package types_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
)

func TestPrecompiledCallsEncoding(t *testing.T) {
	t.Run("test latest version of encoding", func(t *testing.T) {
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
				Address:          testutils.RandomAddress(t),
				RequiredGasCalls: []uint64{2},
				RunCalls:         []types.RunCall{},
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
				Address:          testutils.RandomAddress(t),
				RequiredGasCalls: []uint64{2},
				RunCalls: []types.RunCall{
					{
						Output: []byte{5, 6},
					},
					{
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

	})

	t.Run("test latest version of encoding v1", func(t *testing.T) {
		encodedV1, err := hex.DecodeString("01f7f69408190002143239aaaed0d52d4a5bf218d62453ffc3c20102dcc782030482050680d3820304808e536f6d65206572726f72206d7367")
		require.NoError(t, err)
		expected := types.AggregatedPrecompiledCalls{
			types.PrecompiledCalls{
				Address:          types.Address{0x8, 0x19, 0x0, 0x2, 0x14, 0x32, 0x39, 0xaa, 0xae, 0xd0, 0xd5, 0x2d, 0x4a, 0x5b, 0xf2, 0x18, 0xd6, 0x24, 0x53, 0xff},
				RequiredGasCalls: []uint64{2},
				RunCalls: []types.RunCall{
					{
						Output: []byte{5, 6},
					},
					{
						Output:   []byte{},
						ErrorMsg: "Some error msg",
					},
				},
			},
		}

		apc, err := types.AggregatedPrecompileCallsFromEncoded(encodedV1)
		require.NoError(t, err)
		require.Equal(t, len(expected), len(apc))
		for i := range expected {
			require.Equal(t, expected[i].Address, apc[i].Address)
			require.Equal(t, len(expected[i].RequiredGasCalls), len(apc[i].RequiredGasCalls))
			for j := range expected[i].RequiredGasCalls {
				require.Equal(t, expected[i].RequiredGasCalls[j], apc[i].RequiredGasCalls[j])
			}
			require.Equal(t, len(expected[i].RunCalls), len(apc[i].RunCalls))
			for j := range expected[i].RunCalls {
				require.Equal(t, expected[i].RunCalls[j].ErrorMsg, apc[i].RunCalls[j].ErrorMsg)
				require.Equal(t, expected[i].RunCalls[j].Output, apc[i].RunCalls[j].Output)
			}
		}
	})

}

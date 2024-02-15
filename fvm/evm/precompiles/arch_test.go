package precompiles_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
)

func TestArchContract(t *testing.T) {

	t.Run("test block height", func(t *testing.T) {
		address := testutils.RandomAddress(t)
		height := uint64(12)
		pc := precompiles.ArchContract(
			address,
			func() (uint64, error) {
				return height, nil
			},
			nil,
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
	})

	t.Run("test proof verification", func(t *testing.T) {
		proof := testutils.COAOwnershipProofInContextFixture(t)
		pc := precompiles.ArchContract(
			testutils.RandomAddress(t),
			nil,
			func(p *types.COAOwnershipProofInContext) (bool, error) {
				require.Equal(t, proof, p)
				return true, nil
			},
		)

		abiEncodedData, err := precompiles.ABIEncodeProof(proof)
		require.NoError(t, err)

		// add function selector to the input
		input := append(precompiles.ProofVerifierFuncSig.Bytes(), abiEncodedData...)

		expectedGas := precompiles.ProofVerifierBaseGas +
			uint64(len(proof.KeyIndices))*precompiles.ProofVerifierGasMultiplerPerSignature
		require.Equal(t, expectedGas, pc.RequiredGas(input))

		ret, err := pc.Run(input)
		require.NoError(t, err)

		expected := make([]byte, 32)
		expected[31] = 1
		require.Equal(t, expected, ret)
	})
}

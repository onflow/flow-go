package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
)

func TestProof(t *testing.T) {

	proof := testutils.COAOwnershipProofFixture(t)
	encoded, err := proof.Encode()
	require.NoError(t, err)

	ret, err := types.COAOwnershipProofFromEncoded(encoded)
	require.NoError(t, err)
	require.Equal(t, proof, ret)

	count, err := types.COAOwnershipProofSignatureCountFromEncoded(encoded)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

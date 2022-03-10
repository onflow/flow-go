package packer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/packer"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEncodeDecodeIdentities(t *testing.T) {
	fullIdentities := unittest.IdentifierListFixture(20)
	for s := 0; s < 20; s++ {
		for e := s; e < 20; e++ {
			signers := fullIdentities[s:e]
			indices, err := packer.EncodeSignerIdentifiersToIndices(fullIdentities, signers)
			require.NoError(t, err)

			decoded, err := packer.DecodeSignerIdentifiersFromIndices(fullIdentities, indices)
			require.NoError(t, err)
			require.Equal(t, signers, decoded)
		}
	}
}

func TestEncodeFail(t *testing.T) {
	fullIdentities := unittest.IdentifierListFixture(20)
	_, err := packer.EncodeSignerIdentifiersToIndices(fullIdentities[1:], fullIdentities[:10])
	require.Error(t, err)
}

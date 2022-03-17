package signature_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeIdentities(t *testing.T) {
	fullIdentities := unittest.IdentifierListFixture(20)
	for s := 0; s < 20; s++ {
		for e := s; e < 20; e++ {
			var signers flow.IdentifierList = fullIdentities[s:e]
			indices, err := signature.EncodeSignersToIndices(fullIdentities, signers)
			require.NoError(t, err)

			decoded, err := signature.DecodeSignerIndicesToIdentifiers(fullIdentities, indices)

			fmt.Println(indices)
			fmt.Println(decoded)

			require.NoError(t, err)
			require.Equal(t, signers, decoded)
		}
	}
}

func TestEncodeIdentity(t *testing.T) {
	only := unittest.IdentifierListFixture(1)
	indices, err := packer.EncodeSignerIdentifiersToIndices(only, only)
	require.NoError(t, err)
	// byte(1,0,0,0,0,0,0,0)
	require.Equal(t, []byte{byte(1 << 7)}, indices)
}

func TestEncodeFail(t *testing.T) {
	fullIdentities := unittest.IdentifierListFixture(20)
	_, err := signature.EncodeSignersToIndices(fullIdentities[1:], fullIdentities[:10])
	require.Error(t, err)
}

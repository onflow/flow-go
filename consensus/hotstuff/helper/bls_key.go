package helper

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
)

func MakeBLSKey(t *testing.T) crypto.PrivateKey {
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	privKey, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)
	return privKey
}

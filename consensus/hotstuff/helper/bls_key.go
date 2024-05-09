package helper

import (
	"crypto/rand"
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/require"
)

func MakeBLSKey(t *testing.T) crypto.PrivateKey {
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLen)
	require.NoError(t, err)
	privKey, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)
	return privKey
}

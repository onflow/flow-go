package helper

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
)

func MakeBLSKey(t *testing.T) crypto.PrivateKey {
	seed := make([]byte, crypto.KeyGenSeedMinLenBLS_BLS12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLenBLS_BLS12381)
	require.NoError(t, err)
	privKey, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, seed)
	require.NoError(t, err)
	return privKey
}

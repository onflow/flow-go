package helper

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
)

func MakeBLSKey(t *testing.T) crypto.PrivateKey {
	seed := make([]byte, crypto.KeyGenSeedMinLenBlsBls12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLenBlsBls12381)
	require.NoError(t, err)
	privKey, err := crypto.GeneratePrivateKey(crypto.BlsBls12381, seed)
	require.NoError(t, err)
	return privKey
}

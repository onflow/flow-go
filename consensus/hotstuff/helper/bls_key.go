package helper

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
)

func MakeBLSKey(t *testing.T) crypto.PrivateKey {
	seed := make([]byte, 48)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	privKey, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, seed)
	require.NoError(t, err)
	return privKey
}

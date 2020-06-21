// +build relic

package crypto

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSPOCKProveVerifyAgainstData(t *testing.T) {
	// test the consistency with different inputs
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	input := make([]byte, 100)

	loops := 20
	for j := 0; j < loops; j++ {
		n, err := rand.Read(seed)
		require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(BLSBLS12381, seed)
		require.NoError(t, err)
		_, err = rand.Read(input)
		require.NoError(t, err)
		// SPoCK proof
		s, err := SPOCKProve(sk, input)
		require.NoError(t, err)
		pk := sk.PublicKey()
		// SPoCK verify against the data
		result, err := SPOCKVerifyAgainstData(s, pk, input)
		require.NoError(t, err)
		assert.True(t, result, fmt.Sprintf(
			"Verification should succeed:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
		// test with a different message
		input[0] ^= 1
		result, err = SPOCKVerifyAgainstData(s, pk, input)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should fail:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
		input[0] ^= 1
		// test with a valid but different key
		seed[0] ^= 1
		wrongSk, err := GeneratePrivateKey(BLSBLS12381, seed)
		require.NoError(t, err)
		result, err = SPOCKVerifyAgainstData(s, wrongSk.PublicKey(), input)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should fail:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
	}
}

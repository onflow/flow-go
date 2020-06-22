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
		result, err := SPOCKVerifyAgainstData(pk, s, input)
		require.NoError(t, err)
		assert.True(t, result, fmt.Sprintf(
			"Verification should succeed:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
		// test with a different message
		input[0] ^= 1
		result, err = SPOCKVerifyAgainstData(pk, s, input)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should fail:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
		input[0] ^= 1
		// test with a valid but different key
		seed[0] ^= 1
		wrongSk, err := GeneratePrivateKey(BLSBLS12381, seed)
		require.NoError(t, err)
		result, err = SPOCKVerifyAgainstData(wrongSk.PublicKey(), s, input)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should fail:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
	}
}

func TestSPOCKProveVerify(t *testing.T) {
	// test the consistency with different inputs
	seed1 := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	seed2 := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	input := make([]byte, 100)

	loops := 20
	for j := 0; j < loops; j++ {
		// data
		_, err := rand.Read(input)
		require.NoError(t, err)
		// sk1
		n, err := rand.Read(seed1)
		require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
		require.NoError(t, err)
		sk1, err := GeneratePrivateKey(BLSBLS12381, seed1)
		require.NoError(t, err)
		// sk2
		n, err = rand.Read(seed2)
		require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
		require.NoError(t, err)
		sk2, err := GeneratePrivateKey(BLSBLS12381, seed2)
		require.NoError(t, err)

		// SPoCK proofs
		pr1, err := SPOCKProve(sk1, input)
		require.NoError(t, err)
		pr2, err := SPOCKProve(sk2, input)
		require.NoError(t, err)
		// SPoCK verify against the data
		result, err := SPOCKVerify(sk1.PublicKey(), pr1, sk2.PublicKey(), pr2)
		require.NoError(t, err)
		assert.True(t, result, fmt.Sprintf(
			"Verification should succeed:\n proofs:%s\n %s\n private keys:%s\n %s\n data:%s",
			pr1, pr2, sk1, sk2, input))
		// test with a different message
		input[0] ^= 1
		pr2bis, err := SPOCKProve(sk2, input)
		require.NoError(t, err)
		result, err = SPOCKVerify(sk1.PublicKey(), pr1, sk2.PublicKey(), pr2bis)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should succeed:\n proofs:%s\n %s\n private keys:%s\n %s \n data:%s",
			pr1, pr2bis, sk1, sk2, input))
		input[0] ^= 1
		// test with a different key
		seed2[0] ^= 1
		sk2bis, err := GeneratePrivateKey(BLSBLS12381, seed2)
		require.NoError(t, err)
		result, err = SPOCKVerify(sk1.PublicKey(), pr1, sk2bis.PublicKey(), pr2)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should succeed:\n proofs:%s\n %s\n private keys:%s\n %s \n data:%s",
			pr1, pr2, sk1, sk2bis, input))
	}
}

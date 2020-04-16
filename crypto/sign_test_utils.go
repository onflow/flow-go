package crypto

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

// tests sign and verify are consistent (signature correctness) for several generated keys
func testGenSignVerify(t *testing.T, salg SigningAlgorithm, halg hash.Hasher) {
	// make sure the length is larger than minimum lengths of all the signaure algos
	seedMinLength := 48
	seed := make([]byte, seedMinLength)
	input := make([]byte, 100)

	loops := 1
	for j := 0; j < loops; j++ {
		n, err := rand.Read(seed)
		require.Equal(t, n, seedMinLength)
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(salg, seed)
		require.NoError(t, err)
		_, err = rand.Read(input)
		require.NoError(t, err)
		s, err := sk.Sign(input, halg)
		require.NoError(t, err)
		pk := sk.PublicKey()
		// test a valid signature
		result, err := pk.Verify(s, input, halg)
		require.NoError(t, err)
		assert.True(t, result, fmt.Sprintf(
			"Verification should succeed:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
		// test an invalid signature
		input[0] ^= 1
		result, err = pk.Verify(s, input, halg)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should succeed:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
	}
}

func benchSign(b *testing.B, algo SigningAlgorithm, halg hash.Hasher) {
	seed := make([]byte, 48)
	for j := 0; j < len(seed); j++ {
		seed[j] = byte(j)
	}
	sk, _ := GeneratePrivateKey(algo, seed)

	input := []byte("Bench input")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sk.Sign(input, halg)
	}
	b.StopTimer()
}

func benchVerify(b *testing.B, algo SigningAlgorithm, halg hash.Hasher) {
	seed := make([]byte, 48)
	for j := 0; j < len(seed); j++ {
		seed[j] = byte(j)
	}
	sk, _ := GeneratePrivateKey(algo, seed)
	pk := sk.PublicKey()

	input := []byte("Bench input")
	s, _ := sk.Sign(input, halg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pk.Verify(s, input, halg)
	}

	b.StopTimer()
}

func testKeysAlgorithm(t *testing.T, sk PrivateKey, salg SigningAlgorithm) {
	alg := sk.Algorithm()
	assert.Equal(t, alg, salg)
	alg = sk.PublicKey().Algorithm()
	assert.Equal(t, alg, salg)
}

func testKeySize(t *testing.T, sk PrivateKey, skLen int, pkLen int) {
	size := sk.Size()
	assert.Equal(t, size, skLen)
	size = sk.PublicKey().Size()
	assert.Equal(t, size, pkLen)
}

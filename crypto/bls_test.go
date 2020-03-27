// +build relic

package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BLS tests
func TestBLS_BLS12381(t *testing.T) {
	h, err := NewHasher(SHA3_384)
	require.Nil(t, err)
	seed := h.ComputeHash([]byte{1, 2, 3, 4})
	sk, err := GeneratePrivateKey(BLS_BLS12381, seed)
	require.Nil(t, err)
	halg := NewBLS_KMAC("test tag")
	input := []byte("test input")
	// test the consistency with different inputs
	for i := 0; i < 256; i++ {
		input[0] = byte(i)
		testSignVerify(t, halg, sk, input)
	}
}

// Signing bench
func BenchmarkBLS_BLS12381Sign(b *testing.B) {
	halg := NewBLS_KMAC("bench tag")
	benchSign(b, BLS_BLS12381, halg)
}

// Verifying bench
func BenchmarkBLS_BLS12381Verify(b *testing.B) {
	halg := NewBLS_KMAC("bench tag")
	benchVerify(b, BLS_BLS12381, halg)
}

// TestEncDecPrivateKey tests encoding and decoding of BLS private keys
func TestBLSEquals(t *testing.T) {
	// generate a key pair
	h, err := NewHasher(SHA3_384)
	require.NoError(t, err)
	seed := h.ComputeHash([]byte{1, 2, 3, 4})
	// first pair
	sk1, err := GeneratePrivateKey(BLS_BLS12381, seed)
	require.NoError(t, err)
	pk1 := sk1.PublicKey()
	// second pair
	sk2, err := GeneratePrivateKey(BLS_BLS12381, seed)
	require.NoError(t, err)
	pk2 := sk2.PublicKey()
	// unrelated algo pair
	sk3, err := GeneratePrivateKey(ECDSA_P256, seed)
	require.NoError(t, err)
	pk3 := sk3.PublicKey()
	// tests
	assert.True(t, sk1.Equals(sk2))
	assert.True(t, pk1.Equals(pk2))
	assert.False(t, sk1.Equals(sk3))
	assert.False(t, pk1.Equals(pk3))
}

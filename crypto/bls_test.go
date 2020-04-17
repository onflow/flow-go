// +build relic

package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// BLS tests
func TestBlsBls12381(t *testing.T) {
	seed := make([]byte, KeyGenSeedMinLenBlsBls12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBlsBls12381)
	require.NoError(t, err)
	halg := NewBLS_KMAC("test tag")
	// test the consistency with different inputs
	testGenSignVerify(t, BlsBls12381, halg)
}

// Signing bench
func BenchmarkBlsBls12381Sign(b *testing.B) {
	halg := NewBLS_KMAC("bench tag")
	benchSign(b, BlsBls12381, halg)
}

// Verifying bench
func BenchmarkBlsBls12381Verify(b *testing.B) {
	halg := NewBLS_KMAC("bench tag")
	benchVerify(b, BlsBls12381, halg)
}

// TestBLSEncodeDecode tests encoding and decoding of BLS keys
func TestBLSEncodeDecode(t *testing.T) {
	testEncodeDecode(t, BlsBls12381)
}

// TestBLSEquals tests equal for BLS keys
func TestBLSEquals(t *testing.T) {
	testEquals(t, BlsBls12381, EcdsaP256)
}

// TestBlsUtils tests some utility functions
func TestBlsUtils(t *testing.T) {
	// generate a key pair
	seed := make([]byte, KeyGenSeedMinLenBlsBls12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBlsBls12381)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(BlsBls12381, seed)
	require.NoError(t, err)
	// test Algorithm()
	testKeysAlgorithm(t, sk, BlsBls12381)
	// test Size()
	testKeySize(t, sk, PrKeyLenBlsBls12381, PubKeyLenBlsBls12381)
}

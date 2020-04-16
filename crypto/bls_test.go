// +build relic

package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
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
	seed := make([]byte, KeyGenSeedMinLenBlsBls12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBlsBls12381)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(BlsBls12381, seed)
	assert.Nil(t, err, "the key generation has failed")

	skBytes := sk.Encode()
	skCheck, err := DecodePrivateKey(BlsBls12381, skBytes)
	require.Nil(t, err, "the key decoding has failed")
	assert.True(t, sk.Equals(skCheck), "key equality check failed")
	skCheckBytes := skCheck.Encode()
	assert.Equal(t, skBytes, skCheckBytes, "keys should be equal")

	pk := sk.PublicKey()
	pkBytes := pk.Encode()
	pkCheck, err := DecodePublicKey(BlsBls12381, pkBytes)
	require.Nil(t, err, "the key decoding has failed")
	assert.True(t, pk.Equals(pkCheck), "key equality check failed")
	pkCheckBytes := pkCheck.Encode()
	assert.Equal(t, pkBytes, pkCheckBytes, "keys should be equal")
}

// TestBLSEquals tests equal for BLS keys
func TestBLSEquals(t *testing.T) {
	// generate a key pair
	seed := make([]byte, KeyGenSeedMinLenBlsBls12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBlsBls12381)
	require.NoError(t, err)
	// first pair
	sk1, err := GeneratePrivateKey(BlsBls12381, seed)
	require.NoError(t, err)
	pk1 := sk1.PublicKey()
	// second pair without changing the seed
	sk2, err := GeneratePrivateKey(BlsBls12381, seed)
	require.NoError(t, err)
	pk2 := sk2.PublicKey()
	// unrelated algo pair
	sk3, err := GeneratePrivateKey(EcdsaP256, seed)
	require.NoError(t, err)
	pk3 := sk3.PublicKey()
	// fourth pair after changing the seed
	n, err = rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBlsBls12381)
	require.NoError(t, err)
	sk4, err := GeneratePrivateKey(BlsBls12381, seed)
	require.NoError(t, err)
	pk4 := sk4.PublicKey()
	// tests
	assert.True(t, sk1.Equals(sk2), "key equality should return true")
	assert.True(t, pk1.Equals(pk2), "key equality should return true")
	assert.False(t, sk1.Equals(sk3), "key equality should return false")
	assert.False(t, pk1.Equals(pk3), "key equality should return false")
	assert.False(t, sk1.Equals(sk4), "key equality should return false")
	assert.False(t, pk1.Equals(pk4), "key equality should return false")
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

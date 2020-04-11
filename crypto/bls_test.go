// +build relic

package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BLS tests
func TestBLS_BLS12381(t *testing.T) {
	seed := make([]byte, KeyGenSeedMinLenBlsBls12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBlsBls12381)
	require.NoError(t, err)
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

// TestBLSEncodeDecode tests encoding and decoding of BLS keys
func TestBLSEncodeDecode(t *testing.T) {
	seed := make([]byte, KeyGenSeedMinLenBlsBls12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBlsBls12381)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(BLS_BLS12381, seed)
	assert.Nil(t, err, "the key generation has failed")

	skBytes, err := sk.Encode()
	require.Nil(t, err, "the key encoding has failed")
	skCheck, err := DecodePrivateKey(BLS_BLS12381, skBytes)
	require.Nil(t, err, "the key decoding has failed")
	assert.True(t, sk.Equals(skCheck), "key equality check failed")
	skCheckBytes, err := skCheck.Encode()
	require.Nil(t, err, "the key encoding has failed")
	assert.Equal(t, skBytes, skCheckBytes, "keys should be equal")

	pk := sk.PublicKey()
	pkBytes, err := pk.Encode()
	require.Nil(t, err, "the key encoding has failed")
	pkCheck, err := DecodePublicKey(BLS_BLS12381, pkBytes)
	require.Nil(t, err, "the key decoding has failed")
	assert.True(t, pk.Equals(pkCheck), "key equality check failed")
	pkCheckBytes, err := pkCheck.Encode()
	require.Nil(t, err, "the key encoding has failed")
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
	sk1, err := GeneratePrivateKey(BLS_BLS12381, seed)
	require.NoError(t, err)
	pk1 := sk1.PublicKey()
	// second pair without changing the seed
	sk2, err := GeneratePrivateKey(BLS_BLS12381, seed)
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
	sk4, err := GeneratePrivateKey(BLS_BLS12381, seed)
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

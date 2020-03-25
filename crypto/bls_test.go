// +build relic

package crypto

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BLS tests
func TestBLS_BLS12381(t *testing.T) {
	seed := []byte{1, 2, 3, 4}
	h, _ := NewHasher(SHA3_384)
	sk, err := GeneratePrivateKey(BLS_BLS12381, h.ComputeHash(seed))
	if err != nil {
		log.Error(err.Error())
		return
	}
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
func TestEncDecPrivateKey(t *testing.T) {
	// generate a key pair
	seed := []byte{1, 2, 3, 4}
	h, _ := NewHasher(SHA3_384)
	sk, err := GeneratePrivateKey(BLS_BLS12381, h.ComputeHash(seed))
	require.NoError(t, err)
	// encode the private key
	skBytes, err := sk.Encode()
	require.NoError(t, err)
	// decode the private key
	skCopy, err := DecodePrivateKey(BLS_BLS12381, skBytes)
	require.Nil(t, err)
	assert.True(t, sk.Equals(skCopy), "key equality check failed")
	// check the encode and decode are consistent
	skCopyBytes, err := skCopy.Encode()
	require.NoError(t, err)
	assert.Equal(t, skBytes, skCopyBytes)
}

// TestEncDecPublicKey checks:
// - the consistency of encode/decode of BLS public keys
// - the validity of membership checks of BLS public keys on BLS12-381
//    when decoding a public key
func TestEncDecPublicKey(t *testing.T) {
	// generate a key pair
	seed := []byte{1, 2, 3, 4}
	h, _ := NewHasher(SHA3_384)
	sk, err := GeneratePrivateKey(BLS_BLS12381, h.ComputeHash(seed))
	require.NoError(t, err)
	pk := sk.PublicKey()
	// encode the publick key
	pkBytes, err := pk.Encode()
	require.NoError(t, err)
	// decode the public key including a membership check
	pkCopy, err := DecodePublicKey(BLS_BLS12381, pkBytes)
	// membership check should be valid
	assert.Nil(t, err)
	assert.True(t, pk.Equals(pkCopy), "key equality check failed")
	// check the encode and decode are consistent
	pkCopyBytes, err := pkCopy.Encode()
	require.NoError(t, err)
	assert.Equal(t, pkBytes, pkCopyBytes)
	// check an invalid membership check
	pkBytes[5] ^= 1 // alter one bit
	pkCopy, err = DecodePublicKey(BLS_BLS12381, pkBytes)
	assert.NotNil(t, err)
}

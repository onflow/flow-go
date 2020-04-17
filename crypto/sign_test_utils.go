package crypto

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

// tests sign and verify are consistent for multiple generated keys and messages
func testGenSignVerify(t *testing.T, salg SigningAlgorithm, halg hash.Hasher) {
	t.Logf("Testing Generation/Signature/Verification for %s", salg)
	// make sure the length is larger than minimum lengths of all the signaure algos
	seedMinLength := 48
	seed := make([]byte, seedMinLength)
	input := make([]byte, 100)

	loops := 50
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
		// test with a different message
		input[0] ^= 1
		result, err = pk.Verify(s, input, halg)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should fail:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
		input[0] ^= 1
		// test with a valid but different key
		seed[0] ^= 1
		wrongSk, err := GeneratePrivateKey(salg, seed)
		require.NoError(t, err)
		result, err = wrongSk.PublicKey().Verify(s, input, halg)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should fail:\n signature:%s\n message:%s\n private key:%s", s, input, sk))
	}
}

func testEncodeDecode(t *testing.T, salg SigningAlgorithm) {
	t.Logf("Testing encode/decode for %s", salg)
	// make sure the length is larger than minimum lengths of all the signaure algos
	seedMinLength := 48
	// Key generation seed
	seed := make([]byte, seedMinLength)
	read, err := rand.Read(seed)
	require.Equal(t, read, seedMinLength)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(salg, seed)
	assert.Nil(t, err, "the key generation has failed")

	skBytes := sk.Encode()
	skCheck, err := DecodePrivateKey(salg, skBytes)
	require.Nil(t, err, "the key decoding has failed")
	assert.True(t, sk.Equals(skCheck), "key equality check failed")
	skCheckBytes := skCheck.Encode()
	assert.Equal(t, skBytes, skCheckBytes, "keys should be equal")

	pk := sk.PublicKey()
	pkBytes := pk.Encode()
	pkCheck, err := DecodePublicKey(salg, pkBytes)
	require.Nil(t, err, "the key decoding has failed")
	assert.True(t, pk.Equals(pkCheck), "key equality check failed")
	pkCheckBytes := pkCheck.Encode()
	assert.Equal(t, pkBytes, pkCheckBytes, "keys should be equal")
}

func testEquals(t *testing.T, salg SigningAlgorithm, otherSigAlgo SigningAlgorithm) {
	t.Logf("Testing Equals for %s", salg)
	// make sure the length is larger than minimum lengths of all the signaure algos
	seedMinLength := 48
	// generate a key pair
	seed := make([]byte, seedMinLength)
	n, err := rand.Read(seed)
	require.Equal(t, n, seedMinLength)
	require.NoError(t, err)
	// first pair
	sk1, err := GeneratePrivateKey(salg, seed)
	require.NoError(t, err)
	pk1 := sk1.PublicKey()
	// second pair without changing the seed
	sk2, err := GeneratePrivateKey(salg, seed)
	require.NoError(t, err)
	pk2 := sk2.PublicKey()
	// unrelated algo pair
	sk3, err := GeneratePrivateKey(otherSigAlgo, seed)
	require.NoError(t, err)
	pk3 := sk3.PublicKey()
	// fourth pair with same algo but a different seed
	seed[0] ^= 1
	sk4, err := GeneratePrivateKey(salg, seed)
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

func testKeysAlgorithm(t *testing.T, sk PrivateKey, salg SigningAlgorithm) {
	t.Logf("Testing key.Algorithm for %s", salg)
	alg := sk.Algorithm()
	assert.Equal(t, alg, salg)
	alg = sk.PublicKey().Algorithm()
	assert.Equal(t, alg, salg)
}

func testKeySize(t *testing.T, sk PrivateKey, skLen int, pkLen int) {
	t.Logf("Testing key.Size for %s", sk.Algorithm())
	size := sk.Size()
	assert.Equal(t, size, skLen)
	size = sk.PublicKey().Size()
	assert.Equal(t, size, pkLen)
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

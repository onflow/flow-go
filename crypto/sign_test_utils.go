package crypto

import (
	"fmt"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto/hash"
)

// tests sign and verify are consistent for multiple generated keys and messages
func testGenSignVerify(t *testing.T, salg SigningAlgorithm, halg hash.Hasher) {
	t.Logf("Testing Generation/Signature/Verification for %s", salg)
	// make sure the length is larger than minimum lengths of all the signaure algos
	seedMinLength := 48
	seed := make([]byte, seedMinLength)
	input := make([]byte, 100)
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)

	loops := 50
	for j := 0; j < loops; j++ {
		n, err := mrand.Read(seed)
		require.Equal(t, n, seedMinLength)
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(salg, seed)
		require.NoError(t, err)
		_, err = mrand.Read(input)
		require.NoError(t, err)
		s, err := sk.Sign(input, halg)
		require.NoError(t, err)
		pk := sk.PublicKey()

		// test a valid signature
		result, err := pk.Verify(s, input, halg)
		require.NoError(t, err)
		assert.True(t, result, fmt.Sprintf(
			"Verification should succeed:\n signature:%s\n message:%x\n private key:%s", s, input, sk))

		// test with a different message
		input[0] ^= 1
		result, err = pk.Verify(s, input, halg)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should fail:\n signature:%s\n message:%x\n private key:%s", s, input, sk))
		input[0] ^= 1

		// test with a valid but different key
		seed[0] ^= 1
		wrongSk, err := GeneratePrivateKey(salg, seed)
		require.NoError(t, err)
		result, err = wrongSk.PublicKey().Verify(s, input, halg)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should fail:\n signature:%s\n message:%x\n private key:%s", s, input, sk))

		// test a wrong signature length
		invalidLen := mrand.Intn(2 * len(s)) // try random invalid lengths
		if invalidLen == len(s) {            // map to an invalid length
			invalidLen = 0
		}
		invalidSig := make([]byte, invalidLen)
		result, err = pk.Verify(invalidSig, input, halg)
		require.NoError(t, err)
		assert.False(t, result, fmt.Sprintf(
			"Verification should fail:\n signature:%s\n with invalid length %d", invalidSig, invalidLen))
	}
}

func testKeyGenSeed(t *testing.T, salg SigningAlgorithm, minLen int, maxLen int) {
	// valid seed lengths
	seed := make([]byte, minLen)
	_, err := GeneratePrivateKey(salg, seed)
	assert.NoError(t, err)
	seed = make([]byte, maxLen)
	_, err = GeneratePrivateKey(salg, seed)
	assert.NoError(t, err)
	// invalid seed lengths
	seed = make([]byte, minLen-1)
	_, err = GeneratePrivateKey(salg, seed)
	assert.Error(t, err)
	assert.True(t, IsInvalidInputsError(err))
	seed = make([]byte, maxLen+1)
	_, err = GeneratePrivateKey(salg, seed)
	assert.Error(t, err)
	assert.True(t, IsInvalidInputsError(err))
}

func testEncodeDecode(t *testing.T, salg SigningAlgorithm) {
	t.Logf("Testing encode/decode for %s", salg)
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)
	// make sure the length is larger than minimum lengths of all the signaure algos
	seedMinLength := 48

	loops := 50
	for j := 0; j < loops; j++ {
		// generate a private key
		seed := make([]byte, seedMinLength)
		read, err := mrand.Read(seed)
		require.Equal(t, read, seedMinLength)
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(salg, seed)
		assert.Nil(t, err, "the key generation has failed")
		seed[0] ^= 1 // alter the seed to get a new private key
		distinctSk, err := GeneratePrivateKey(salg, seed)
		require.NoError(t, err)

		// check private key encoding
		skBytes := sk.Encode()
		skCheck, err := DecodePrivateKey(salg, skBytes)
		require.Nil(t, err, "the key decoding has failed")
		assert.True(t, sk.Equals(skCheck), "key equality check failed")
		skCheckBytes := skCheck.Encode()
		assert.Equal(t, skBytes, skCheckBytes, "keys should be equal")
		distinctSkBytes := distinctSk.Encode()
		assert.NotEqual(t, skBytes, distinctSkBytes, "keys should be different")

		// check public key encoding
		pk := sk.PublicKey()
		pkBytes := pk.Encode()
		pkCheck, err := DecodePublicKey(salg, pkBytes)
		require.Nil(t, err, "the key decoding has failed")
		assert.True(t, pk.Equals(pkCheck), "key equality check failed")
		pkCheckBytes := pkCheck.Encode()
		assert.Equal(t, pkBytes, pkCheckBytes, "keys should be equal")
		distinctPkBytes := distinctSk.PublicKey().Encode()
		assert.NotEqual(t, pkBytes, distinctPkBytes, "keys should be different")

		// same for the compressed encoding
		pkComprBytes := pk.EncodeCompressed()
		pkComprCheck, err := DecodePublicKeyCompressed(salg, pkComprBytes)
		require.Nil(t, err, "the key decoding has failed")
		assert.True(t, pk.Equals(pkComprCheck), "key equality check failed")
		pkCheckComprBytes := pkComprCheck.EncodeCompressed()
		assert.Equal(t, pkComprBytes, pkCheckComprBytes, "keys should be equal")
		distinctPkComprBytes := distinctSk.PublicKey().EncodeCompressed()
		assert.NotEqual(t, pkComprBytes, distinctPkComprBytes, "keys should be different")
	}

	// test invalid private keys (equal to the curve group order)
	groupOrder := make(map[SigningAlgorithm][]byte)
	groupOrder[ECDSAP256] = []byte{255, 255, 255, 255, 0, 0, 0, 0, 255, 255, 255,
		255, 255, 255, 255, 255, 188, 230, 250, 173, 167,
		23, 158, 132, 243, 185, 202, 194, 252, 99, 37, 81}

	groupOrder[ECDSASecp256k1] = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 254, 186, 174, 220, 230,
		175, 72, 160, 59, 191, 210, 94, 140, 208, 54, 65, 65}

	groupOrder[BLSBLS12381] = []byte{0x73, 0xED, 0xA7, 0x53, 0x29, 0x9D, 0x7D, 0x48, 0x33, 0x39,
		0xD8, 0x08, 0x09, 0xA1, 0xD8, 0x05, 0x53, 0xBD, 0xA4, 0x02, 0xFF, 0xFE,
		0x5B, 0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x01}
	_, err := DecodePrivateKey(salg, groupOrder[salg])
	require.Error(t, err, "the key decoding should fail - private key value is too large")
	assert.True(t, IsInvalidInputsError(err))
}

func testEquals(t *testing.T, salg SigningAlgorithm, otherSigAlgo SigningAlgorithm) {
	t.Logf("Testing Equals for %s", salg)
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)
	// make sure the length is larger than minimum lengths of all the signaure algos
	seedMinLength := 48

	// generate a key pair
	seed := make([]byte, seedMinLength)
	n, err := mrand.Read(seed)
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

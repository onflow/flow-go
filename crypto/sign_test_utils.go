package crypto

import (
	crand "crypto/rand"
	"fmt"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto/hash"
)

func getPRG(t *testing.T) *mrand.Rand {
	random := time.Now().UnixNano()
	t.Logf("rng seed is %d", random)
	rng := mrand.New(mrand.NewSource(random))
	return rng
}

func TestKeyGenErrors(t *testing.T) {
	seed := make([]byte, 50)
	invalidSigAlgo := SigningAlgorithm(20)
	sk, err := GeneratePrivateKey(invalidSigAlgo, seed)
	assert.Nil(t, sk)
	assert.Error(t, err)
	assert.True(t, IsInvalidInputsError(err))
}

func TestHasherErrors(t *testing.T) {
	t.Run("nilHasher error sanity", func(t *testing.T) {
		err := nilHasherError
		invInpError := invalidInputsErrorf("")
		otherError := fmt.Errorf("some error")
		assert.True(t, IsNilHasherError(err))
		assert.False(t, IsInvalidInputsError(err))
		assert.False(t, IsNilHasherError(invInpError))
		assert.False(t, IsNilHasherError(otherError))
		assert.False(t, IsNilHasherError(nil))
	})

	t.Run("nilHasher error sanity", func(t *testing.T) {
		err := invalidHasherSizeErrorf("")
		invInpError := invalidInputsErrorf("")
		otherError := fmt.Errorf("some error")
		assert.True(t, IsInvalidHasherSizeError(err))
		assert.False(t, IsInvalidInputsError(err))
		assert.False(t, IsInvalidHasherSizeError(invInpError))
		assert.False(t, IsInvalidHasherSizeError(otherError))
		assert.False(t, IsInvalidHasherSizeError(nil))
	})
}

// tests sign and verify are consistent for multiple generated keys and messages
func testGenSignVerify(t *testing.T, salg SigningAlgorithm, halg hash.Hasher) {
	t.Run(fmt.Sprintf("Generation/Signature/Verification for %s", salg), func(t *testing.T) {
		seed := make([]byte, KeyGenSeedMinLen)
		input := make([]byte, 100)
		rand := getPRG(t)

		loops := 50
		for j := 0; j < loops; j++ {
			n, err := rand.Read(seed)
			require.Equal(t, n, KeyGenSeedMinLen)
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
			assert.True(t, result)

			// test with a different message
			input[0] ^= 1
			result, err = pk.Verify(s, input, halg)
			require.NoError(t, err)
			assert.False(t, result)
			input[0] ^= 1

			// test with a valid but different key
			seed[0] ^= 1
			wrongSk, err := GeneratePrivateKey(salg, seed)
			require.NoError(t, err)
			result, err = wrongSk.PublicKey().Verify(s, input, halg)
			require.NoError(t, err)
			assert.False(t, result)

			// test a wrong signature length
			invalidLen := rand.Intn(2 * len(s)) // try random invalid lengths
			if invalidLen == len(s) {           // map to an invalid length
				invalidLen = 0
			}
			invalidSig := make([]byte, invalidLen)
			result, err = pk.Verify(invalidSig, input, halg)
			require.NoError(t, err)
			assert.False(t, result)
		}
	})
}

// tests the key generation constraints with regards to the input seed, mainly
// the seed length constraints and the result determinicity.
func testKeyGenSeed(t *testing.T, salg SigningAlgorithm, minLen int, maxLen int) {
	t.Run("seed length check", func(t *testing.T) {
		// valid seed lengths
		seed := make([]byte, minLen)
		_, err := GeneratePrivateKey(salg, seed)
		assert.NoError(t, err)
		if maxLen > 0 {
			seed = make([]byte, maxLen)
			_, err = GeneratePrivateKey(salg, seed)
			assert.NoError(t, err)
		}
		// invalid seed lengths
		seed = make([]byte, minLen-1)
		_, err = GeneratePrivateKey(salg, seed)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		if maxLen > 0 {
			seed = make([]byte, maxLen+1)
			_, err = GeneratePrivateKey(salg, seed)
			assert.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
		}
	})

	t.Run("deterministic generation", func(t *testing.T) {
		// same seed results in the same key
		seed := make([]byte, minLen)
		read, err := crand.Read(seed)
		require.Equal(t, read, minLen)
		require.NoError(t, err)
		sk1, err := GeneratePrivateKey(salg, seed)
		require.NoError(t, err)
		sk2, err := GeneratePrivateKey(salg, seed)
		require.NoError(t, err)
		assert.True(t, sk1.Equals(sk2))
		// different seed results in a different key
		seed[0] ^= 1 // alter a seed bit
		sk2, err = GeneratePrivateKey(salg, seed)
		require.NoError(t, err)
		assert.False(t, sk1.Equals(sk2))
	})
}

var BLS12381Order = []byte{0x73, 0xED, 0xA7, 0x53, 0x29, 0x9D, 0x7D, 0x48, 0x33, 0x39,
	0xD8, 0x08, 0x09, 0xA1, 0xD8, 0x05, 0x53, 0xBD, 0xA4, 0x02, 0xFF, 0xFE,
	0x5B, 0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x01}

func testEncodeDecode(t *testing.T, salg SigningAlgorithm) {
	t.Run(fmt.Sprintf("generic encode/decode for %s", salg), func(t *testing.T) {
		rand := getPRG(t)

		t.Run("happy path tests", func(t *testing.T) {
			loops := 50
			for j := 0; j < loops; j++ {
				// generate a private key
				seed := make([]byte, KeyGenSeedMinLen)
				read, err := rand.Read(seed)
				require.Equal(t, read, KeyGenSeedMinLen)
				require.NoError(t, err)
				sk, err := GeneratePrivateKey(salg, seed)
				assert.Nil(t, err)
				seed[0] ^= 1 // alter the seed to get a new private key
				distinctSk, err := GeneratePrivateKey(salg, seed)
				require.NoError(t, err)

				// check private key encoding
				skBytes := sk.Encode()
				skCheck, err := DecodePrivateKey(salg, skBytes)
				require.Nil(t, err)
				assert.True(t, sk.Equals(skCheck))
				skCheckBytes := skCheck.Encode()
				assert.Equal(t, skBytes, skCheckBytes)
				distinctSkBytes := distinctSk.Encode()
				assert.NotEqual(t, skBytes, distinctSkBytes)

				// check public key encoding
				pk := sk.PublicKey()
				pkBytes := pk.Encode()
				pkCheck, err := DecodePublicKey(salg, pkBytes)
				require.Nil(t, err)
				assert.True(t, pk.Equals(pkCheck))
				pkCheckBytes := pkCheck.Encode()
				assert.Equal(t, pkBytes, pkCheckBytes)
				distinctPkBytes := distinctSk.PublicKey().Encode()
				assert.NotEqual(t, pkBytes, distinctPkBytes)

				// same for the compressed encoding
				// skip is BLS is used and compression isn't supported
				if !(salg == BLSBLS12381 && !isG2Compressed()) {
					pkComprBytes := pk.EncodeCompressed()
					pkComprCheck, err := DecodePublicKeyCompressed(salg, pkComprBytes)
					require.Nil(t, err)
					assert.True(t, pk.Equals(pkComprCheck))
					pkCheckComprBytes := pkComprCheck.EncodeCompressed()
					assert.Equal(t, pkComprBytes, pkCheckComprBytes)
					distinctPkComprBytes := distinctSk.PublicKey().EncodeCompressed()
					assert.NotEqual(t, pkComprBytes, distinctPkComprBytes)
				}
			}
		})

		// test invalid private keys (equal to the curve group order)

		t.Run("private keys equal to the group order", func(t *testing.T) {
			groupOrder := make(map[SigningAlgorithm][]byte)
			groupOrder[ECDSAP256] = []byte{255, 255, 255, 255, 0, 0, 0, 0, 255, 255, 255,
				255, 255, 255, 255, 255, 188, 230, 250, 173, 167,
				23, 158, 132, 243, 185, 202, 194, 252, 99, 37, 81}

			groupOrder[ECDSASecp256k1] = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
				255, 255, 255, 255, 255, 254, 186, 174, 220, 230,
				175, 72, 160, 59, 191, 210, 94, 140, 208, 54, 65, 65}

			groupOrder[BLSBLS12381] = BLS12381Order

			sk, err := DecodePrivateKey(salg, groupOrder[salg])
			require.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.Nil(t, sk)
		})

		// test invalid private and public keys (invalid length)
		t.Run("invalid key length", func(t *testing.T) {
			// private key
			skLens := make(map[SigningAlgorithm]int)
			skLens[ECDSAP256] = PrKeyLenECDSAP256
			skLens[ECDSASecp256k1] = PrKeyLenECDSASecp256k1
			skLens[BLSBLS12381] = 32

			bytes := make([]byte, skLens[salg]+1)
			sk, err := DecodePrivateKey(salg, bytes)
			require.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.Nil(t, sk)

			// public key
			pkLens := make(map[SigningAlgorithm]int)
			pkLens[ECDSAP256] = PubKeyLenECDSAP256
			pkLens[ECDSASecp256k1] = PubKeyLenECDSASecp256k1
			pkLens[BLSBLS12381] = 96

			bytes = make([]byte, pkLens[salg]+1)
			pk, err := DecodePublicKey(salg, bytes)
			require.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.Nil(t, pk)
		})
	})
}

func testEquals(t *testing.T, salg SigningAlgorithm, otherSigAlgo SigningAlgorithm) {
	t.Run(fmt.Sprintf("equals for %s", salg), func(t *testing.T) {
		rand := getPRG(t)
		// generate a key pair
		seed := make([]byte, KeyGenSeedMinLen)
		n, err := rand.Read(seed)
		require.Equal(t, n, KeyGenSeedMinLen)
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
		assert.True(t, sk1.Equals(sk2))
		assert.True(t, pk1.Equals(pk2))
		assert.False(t, sk1.Equals(sk3))
		assert.False(t, pk1.Equals(pk3))
		assert.False(t, sk1.Equals(sk4))
		assert.False(t, pk1.Equals(pk4))
	})
}

func testKeysAlgorithm(t *testing.T, sk PrivateKey, salg SigningAlgorithm) {
	t.Run(fmt.Sprintf("key.Algorithm for %s", salg), func(t *testing.T) {
		alg := sk.Algorithm()
		assert.Equal(t, alg, salg)
		alg = sk.PublicKey().Algorithm()
		assert.Equal(t, alg, salg)
	})
}

func testKeySize(t *testing.T, sk PrivateKey, skLen int, pkLen int) {
	t.Run(fmt.Sprintf("key.Size for %s", sk.Algorithm()), func(t *testing.T) {
		size := sk.Size()
		assert.Equal(t, size, skLen)
		size = sk.PublicKey().Size()
		assert.Equal(t, size, pkLen)
	})
}

func benchVerify(b *testing.B, algo SigningAlgorithm, halg hash.Hasher) {
	seed := make([]byte, 48)
	for j := 0; j < len(seed); j++ {
		seed[j] = byte(j)
	}
	sk, err := GeneratePrivateKey(algo, seed)
	require.NoError(b, err)
	pk := sk.PublicKey()

	input := []byte("Bench input")
	s, err := sk.Sign(input, halg)
	require.NoError(b, err)
	var result bool

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err = pk.Verify(s, input, halg)
		require.NoError(b, err)
	}
	// sanity check
	require.True(b, result)

	b.StopTimer()
}

func benchSign(b *testing.B, algo SigningAlgorithm, halg hash.Hasher) {
	seed := make([]byte, 48)
	for j := 0; j < len(seed); j++ {
		seed[j] = byte(j)
	}
	sk, err := GeneratePrivateKey(algo, seed)
	require.NoError(b, err)

	input := []byte("Bench input")
	var signature []byte

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		signature, err = sk.Sign(input, halg)
		require.NoError(b, err)
	}
	// sanity check
	result, err := sk.PublicKey().Verify(signature, input, halg)
	require.NoError(b, err)
	require.True(b, result)

	b.StopTimer()
}

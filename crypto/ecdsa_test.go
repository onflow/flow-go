package crypto

import (
	"testing"

	"math/rand"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ECDSA tests
func TestECDSA(t *testing.T) {
	ecdsaCurves := []SigningAlgorithm{
		EcdsaP256,
		EcdsaSecp256k1,
	}
	for i, curve := range ecdsaCurves {
		t.Logf("Testing ECDSA for curve %s", curve)
		halg := hash.NewSHA3_256()
		testGenSignVerify(t, ecdsaCurves[i], halg)
	}
}

// Signing bench
func BenchmarkEcdsaP256Sign(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchSign(b, EcdsaP256, halg)
}

// Verifying bench
func BenchmarkEcdsaP256Verify(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchVerify(b, EcdsaP256, halg)
}

// Signing bench
func BenchmarkEcdsaSecp256k1Sign(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchSign(b, EcdsaSecp256k1, halg)
}

// Verifying bench
func BenchmarkEcdsaSecp256k1Verify(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchVerify(b, EcdsaSecp256k1, halg)
}

// ECDSA tests

// TestBLSEncodeDecode tests encoding and decoding of ECDSA keys
func TestECDSAEncodeDecode(t *testing.T) {
	ecdsaCurves := []SigningAlgorithm{
		EcdsaP256,
		EcdsaSecp256k1,
	}
	ecdsaSeedLen := []int{
		KeyGenSeedMinLenEcdsaP256,
		KeyGenSeedMinLenEcdsaSecp256k1,
	}

	for i, curve := range ecdsaCurves {
		t.Logf("Testing encode/decode for curve %s", curve)
		// Key generation seed
		seed := make([]byte, ecdsaSeedLen[i])
		read, err := rand.Read(seed)
		require.Equal(t, read, ecdsaSeedLen[i])
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(curve, seed)
		assert.Nil(t, err, "the key generation has failed")

		skBytes := sk.Encode()
		skCheck, err := DecodePrivateKey(curve, skBytes)
		require.Nil(t, err, "the key decoding has failed")
		assert.True(t, sk.Equals(skCheck), "key equality check failed")
		skCheckBytes := skCheck.Encode()
		assert.Equal(t, skBytes, skCheckBytes, "keys should be equal")

		pk := sk.PublicKey()
		pkBytes := pk.Encode()
		pkCheck, err := DecodePublicKey(curve, pkBytes)
		require.Nil(t, err, "the key decoding has failed")
		assert.True(t, pk.Equals(pkCheck), "key equality check failed")
		pkCheckBytes := pkCheck.Encode()
		assert.Equal(t, pkBytes, pkCheckBytes, "keys should be equal")
	}
}

// TestECDSAEquals tests equal for ECDSA keys
func TestECDSAEquals(t *testing.T) {
	ecdsaCurves := []SigningAlgorithm{
		EcdsaP256,
		EcdsaSecp256k1,
	}
	ecdsaSeedLen := []int{
		KeyGenSeedMinLenEcdsaP256,
		KeyGenSeedMinLenEcdsaSecp256k1,
	}

	for i, curve := range ecdsaCurves {
		// generate a key pair
		seed := make([]byte, ecdsaSeedLen[i])
		n, err := rand.Read(seed)
		require.Equal(t, n, ecdsaSeedLen[i])
		require.NoError(t, err)
		// first pair
		sk1, err := GeneratePrivateKey(curve, seed)
		require.NoError(t, err)
		pk1 := sk1.PublicKey()
		// second pair
		sk2, err := GeneratePrivateKey(curve, seed)
		require.NoError(t, err)
		pk2 := sk2.PublicKey()
		// third pair of a  different curve
		sk3, err := GeneratePrivateKey(ecdsaCurves[i]^1, seed)
		require.NoError(t, err)
		pk3 := sk3.PublicKey()
		// fourth pair after changing the seed
		n, err = rand.Read(seed)
		require.Equal(t, n, ecdsaSeedLen[i])
		require.NoError(t, err)
		sk4, err := GeneratePrivateKey(curve, seed)
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
}

// TestECDSAUtils tests some utility functions
func TestECDSAUtils(t *testing.T) {
	ecdsaCurves := []SigningAlgorithm{
		EcdsaP256,
		EcdsaSecp256k1,
	}
	ecdsaSeedLen := []int{
		KeyGenSeedMinLenEcdsaP256,
		KeyGenSeedMinLenEcdsaSecp256k1,
	}
	ecdsaPrKeyLen := []int{
		PrKeyLenEcdsaP256,
		PrKeyLenEcdsaSecp256k1,
	}
	ecdsaPubKeyLen := []int{
		PubKeyLenEcdsaP256,
		PubKeyLenEcdsaSecp256k1,
	}

	for i, curve := range ecdsaCurves {
		// generate a key pair
		seed := make([]byte, ecdsaSeedLen[i])
		n, err := rand.Read(seed)
		require.Equal(t, n, ecdsaSeedLen[i])
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(curve, seed)
		require.NoError(t, err)
		testKeysAlgorithm(t, sk, ecdsaCurves[i])
		testKeySize(t, sk, ecdsaPrKeyLen[i], ecdsaPubKeyLen[i])
	}
}

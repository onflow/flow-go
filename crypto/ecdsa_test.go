package crypto

import (
	"testing"

	"math/rand"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/stretchr/testify/require"
)

// ECDSA tests
func TestEcdsa(t *testing.T) {
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
func TestEcdsaEncodeDecode(t *testing.T) {
	ecdsaCurves := []SigningAlgorithm{
		EcdsaP256,
		EcdsaSecp256k1,
	}

	for _, curve := range ecdsaCurves {
		testEncodeDecode(t, curve)
	}
}

// TestEcdsaEquals tests equal for ECDSA keys
func TestEcdsaEquals(t *testing.T) {
	ecdsaCurves := []SigningAlgorithm{
		EcdsaP256,
		EcdsaSecp256k1,
	}
	for i, curve := range ecdsaCurves {
		testEquals(t, curve, ecdsaCurves[i]^1)
	}
}

// TestEcdsaUtils tests some utility functions
func TestEcdsaUtils(t *testing.T) {
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

package crypto

import (
	"testing"

	"math/rand"

	"github.com/dapperlabs/flow-go/crypto/hash"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ECDSA tests
func TestECDSA(t *testing.T) {
	ECDSAcurves := []SigningAlgorithm{
		ECDSA_P256,
		ECDSA_SECp256k1,
	}
	ECDSAseedLen := []int{
		KeyGenSeedMinLenECDSA_P256,
		KeyGenSeedMinLenECDSA_SECp256k1,
	}
	for i, curve := range ECDSAcurves {
		t.Logf("Testing ECDSA for curve %s", curve)

		halg := hash.NewSHA3_256()
		seed := make([]byte, ECDSAseedLen[i])
		rand.Read(seed)
		sk, err := GeneratePrivateKey(curve, seed)
		if err != nil {
			log.Error(err.Error())
			return
		}
		input := []byte("test")
		testSignVerify(t, halg, sk, input)
	}
}

// Signing bench
func BenchmarkECDSA_P256Sign(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchSign(b, ECDSA_P256, halg)
}

// Verifying bench
func BenchmarkECDSA_P256Verify(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchVerify(b, ECDSA_P256, halg)
}

// Signing bench
func BenchmarkECDSA_SECp256k1Sign(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchSign(b, ECDSA_SECp256k1, halg)
}

// Verifying bench
func BenchmarkECDSA_SECp256k1Verify(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchVerify(b, ECDSA_SECp256k1, halg)
}

// ECDSA tests

// TestBLSEncodeDecode tests encoding and decoding of ECDSA keys
func TestECDSAEncodeDecode(t *testing.T) {
	ECDSAcurves := []SigningAlgorithm{
		ECDSA_P256,
		ECDSA_SECp256k1,
	}
	ECDSAseedLen := []int{
		KeyGenSeedMinLenECDSA_P256,
		KeyGenSeedMinLenECDSA_SECp256k1,
	}

	for i, curve := range ECDSAcurves {
		t.Logf("Testing encode/decode for curve %s", curve)
		// Key generation seed
		seed := make([]byte, ECDSAseedLen[i])
		rand.Read(seed)
		sk, err := GeneratePrivateKey(curve, seed)
		assert.Nil(t, err, "the key generation has failed")

		skBytes, err := sk.Encode()
		require.Nil(t, err, "the key encoding has failed")
		skCheck, err := DecodePrivateKey(curve, skBytes)
		require.Nil(t, err, "the key decoding has failed")
		assert.True(t, sk.Equals(skCheck), "key equality check failed")
		skCheckBytes, err := skCheck.Encode()
		require.Nil(t, err, "the key encoding has failed")
		assert.Equal(t, skBytes, skCheckBytes, "keys should be equal")

		pk := sk.PublicKey()
		pkBytes, err := pk.Encode()
		require.Nil(t, err, "the key encoding has failed")
		pkCheck, err := DecodePublicKey(curve, pkBytes)
		require.Nil(t, err, "the key decoding has failed")
		assert.True(t, pk.Equals(pkCheck), "key equality check failed")
		pkCheckBytes, err := pkCheck.Encode()
		require.Nil(t, err, "the key encoding has failed")
		assert.Equal(t, pkBytes, pkCheckBytes, "keys should be equal")
	}
}

// TestECDSAEquals tests equal for ECDSA keys
func TestECDSAEquals(t *testing.T) {
	ECDSAcurves := []SigningAlgorithm{
		ECDSA_P256,
		ECDSA_SECp256k1,
	}
	ECDSAseedLen := []int{
		KeyGenSeedMinLenECDSA_P256,
		KeyGenSeedMinLenECDSA_SECp256k1,
	}

	for i, curve := range ECDSAcurves {
		// generate a key pair
		seed := make([]byte, ECDSAseedLen[i])
		rand.Read(seed)
		// first pair
		sk1, err := GeneratePrivateKey(curve, seed)
		require.NoError(t, err)
		pk1 := sk1.PublicKey()
		// second pair
		sk2, err := GeneratePrivateKey(curve, seed)
		require.NoError(t, err)
		pk2 := sk2.PublicKey()
		// third pair of a  different curve
		sk3, err := GeneratePrivateKey(ECDSAcurves[i]^1, seed)
		require.NoError(t, err)
		pk3 := sk3.PublicKey()
		// fourth pair after changing the seed
		rand.Read(seed)
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

package crypto

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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

		halg := NewSHA3_256()
		seed := make([]byte, ECDSAseedLen[i])
		for j := 0; j < len(seed); j++ {
			seed[j] = byte(j)
		}
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
	halg := NewSHA3_256()
	benchSign(b, ECDSA_P256, halg)
}

// Verifying bench
func BenchmarkECDSA_P256Verify(b *testing.B) {
	halg := NewSHA3_256()
	benchVerify(b, ECDSA_P256, halg)
}

// Signing bench
func BenchmarkECDSA_SECp256k1Sign(b *testing.B) {
	halg := NewSHA3_256()
	benchSign(b, ECDSA_SECp256k1, halg)
}

// Verifying bench
func BenchmarkECDSA_SECp256k1Verify(b *testing.B) {
	halg := NewSHA3_256()
	benchVerify(b, ECDSA_SECp256k1, halg)
}

// ECDSA tests
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
		for j := 0; j < len(seed); j++ {
			seed[j] = byte(j)
		}
		sk, err := GeneratePrivateKey(curve, seed)
		assert.Nil(t, err, "the key generation has failed")

		skBytes, err := sk.Encode()
		assert.Nil(t, err, "the key encoding has failed")
		skCheck, err := DecodePrivateKey(curve, skBytes)
		assert.True(t, sk.Equals(skCheck), "key equality check failed")
		assert.Nil(t, err, "the key decoding has failed")
		skCheckBytes, err := skCheck.Encode()
		assert.Nil(t, err, "the key encoding has failed")
		assert.Equal(t, skBytes, skCheckBytes, "keys should be equal")

		pk := sk.PublicKey()
		pkBytes, err := pk.Encode()
		assert.Nil(t, err, "the key encoding has failed")
		pkCheck, err := DecodePublicKey(curve, pkBytes)
		assert.True(t, pk.Equals(pkCheck), "key equality check failed")
		assert.Nil(t, err, "the key decoding has failed")
		pkCheckBytes, err := pkCheck.Encode()
		assert.Nil(t, err, "the key encoding has failed")
		assert.Equal(t, pkBytes, pkCheckBytes, "keys should be equal")
	}
}

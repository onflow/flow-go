package crypto

import (
	"testing"

	log "github.com/sirupsen/logrus"
)

// ECDSA tests
func TestECDSA(t *testing.T) {
	ECDSAcurves := []SigningAlgorithm{ECDSA_P256, ECDSA_SECp256k1}
	for _, curve := range ECDSAcurves {
		t.Logf("Testing ECDSA for curve %s", curve)

		halg, err := NewHasher(SHA3_256)
		if err != nil {
			log.Error(err.Error())
			return
		}
		seed := []byte{1, 2, 3, 4}
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
	halg, _ := NewHasher(SHA3_256)
	benchSign(b, ECDSA_P256, halg)
}

// Verifying bench
func BenchmarkECDSA_P256Verify(b *testing.B) {
	halg, _ := NewHasher(SHA3_256)
	benchVerify(b, ECDSA_P256, halg)
}

// Signing bench
func BenchmarkECDSA_SECp256k1Sign(b *testing.B) {
	halg, _ := NewHasher(SHA3_256)
	benchSign(b, ECDSA_SECp256k1, halg)
}

// Verifying bench
func BenchmarkECDSA_SECp256k1Verify(b *testing.B) {
	halg, _ := NewHasher(SHA3_256)
	benchVerify(b, ECDSA_SECp256k1, halg)
}

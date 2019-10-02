package crypto

import (
	"testing"

	log "github.com/sirupsen/logrus"
)

func testSignVerify(t *testing.T, halg Hasher, sk PrivateKey, input []byte) {
	s, err := sk.Sign(input, halg)
	if err != nil {
		log.Error(err.Error())
		return
	}
	pk := sk.Pubkey()
	result, err := pk.Verify(s, input, halg)
	if err != nil {
		log.Error(err.Error())
		return
	}
	if result == false {
		t.Errorf("Verification failed:\n signature is %s", s)
	} else {
		t.Logf("Verification passed:\n signature is %s", s)
	}
}

func benchSign(b *testing.B, salg Signer, halg Hasher) {
	seed := []byte("keyseed")
	sk, _ := salg.GeneratePrKey(seed)

	input := []byte("Bench input")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sk.Sign(input, halg)
	}
	b.StopTimer()
}

func benchVerify(b *testing.B, salg Signer, halg Hasher) {
	seed := []byte("keyseed")
	sk, _ := salg.GeneratePrKey(seed)
	pk := sk.Pubkey()

	input := []byte("Bench input")
	s, _ := sk.Sign(input, halg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pk.Verify(s, input, halg)
	}

	b.StopTimer()
}

// BLS tests
func TestBLS_BLS12381(t *testing.T) {
	salg, err := NewSigner(BLS_BLS12381)
	if err != nil {
		log.Error(err.Error())
		return
	}
	seed := []byte{1, 2, 3, 4}
	sk, err := salg.GeneratePrKey(seed)
	if err != nil {
		log.Error(err.Error())
		return
	}
	halg, err := NewHasher(SHA3_384)
	input := []byte("test")
	testSignVerify(t, halg, sk, input)
}

// Signing bench
func BenchmarkBLS_BLS12381Sign(b *testing.B) {
	salg, err := NewSigner(BLS_BLS12381)
	if err != nil {
		log.Error(err.Error())
		return
	}
	benchSign(b, salg, nil)
}

// Verifying bench
func BenchmarkBLS_BLS12381Verify(b *testing.B) {
	salg, err := NewSigner(BLS_BLS12381)
	if err != nil {
		log.Error(err.Error())
		return
	}
	benchVerify(b, salg, nil)
}

// ECDSA tests
func TestECDSA(t *testing.T) {
	ECDSAcurves := []AlgoName{ECDSA_P256, ECDSA_SECp256k1}
	for _, curve := range ECDSAcurves {
		t.Logf("Testing ECDSA for curve %s", curve)
		salg, err := NewSigner(curve)
		if err != nil {
			log.Error(err.Error())
			return
		}
		halg, err := NewHasher(SHA3_256)
		if err != nil {
			log.Error(err.Error())
			return
		}
		seed := []byte{1, 2, 3, 4}
		sk, err := salg.GeneratePrKey(seed)
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
	salg, _ := NewSigner(ECDSA_P256)
	halg, _ := NewHasher(SHA3_256)
	benchSign(b, salg, halg)
}

// Verifying bench
func BenchmarkECDSA_P256Verify(b *testing.B) {
	salg, _ := NewSigner(ECDSA_P256)
	halg, _ := NewHasher(SHA3_256)
	benchVerify(b, salg, halg)
}

// Signing bench
func BenchmarkECDSA_SECp256k1Sign(b *testing.B) {
	salg, _ := NewSigner(ECDSA_SECp256k1)
	halg, _ := NewHasher(SHA3_256)
	benchSign(b, salg, halg)
}

// Verifying bench
func BenchmarkECDSA_SECp256k1Verify(b *testing.B) {
	salg, _ := NewSigner(ECDSA_SECp256k1)
	halg, _ := NewHasher(SHA3_256)
	benchVerify(b, salg, halg)
}

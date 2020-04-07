package crypto

import (
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

func testSignVerify(t *testing.T, halg hash.Hasher, sk PrivateKey, input []byte) {
	s, err := sk.Sign(input, halg)
	if err != nil {
		log.Error(err.Error())
		return
	}
	pk := sk.PublicKey()
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

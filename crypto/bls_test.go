// +build relic

package crypto

import (
	"testing"

	log "github.com/sirupsen/logrus"
)

// BLS tests
func TestBLS_BLS12381(t *testing.T) {
	seed := []byte{1, 2, 3, 4}
	sk, err := GeneratePrivateKey(BLS_BLS12381, seed)
	if err != nil {
		log.Error(err.Error())
		return
	}
	halg := NewBlsKmac("test tag")
	input := []byte("test input")
	// test the consistency with different inputs
	for i := 0; i < 256; i++ {
		input[0] = byte(i)
		testSignVerify(t, halg, sk, input)
	}
}

// Signing bench
func BenchmarkBLS_BLS12381Sign(b *testing.B) {
	halg := NewBlsKmac("bench tag")
	benchSign(b, BLS_BLS12381, halg)
}

// Verifying bench
func BenchmarkBLS_BLS12381Verify(b *testing.B) {
	halg := NewBlsKmac("bench tag")
	benchVerify(b, BLS_BLS12381, halg)
}

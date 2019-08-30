package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"

	log "github.com/sirupsen/logrus"
)

// Sanity checks of SHA3_256
func TestSha3_256(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")

	alg, err := NewHashAlgo(SHA3_256)
	if err != nil {
		log.Error(err.Error())
		return
	}
	hash := alg.ComputeBytesHash(input).Bytes()
	checkBytes(t, input, expected, hash)

	hash = alg.ComputeStructHash(&testStruct{"te", "st"}).Bytes()
	checkBytes(t, input, expected, hash)

	alg.Reset()
	alg.AddBytes([]byte("te"))
	alg.AddBytes([]byte("s"))
	alg.AddBytes([]byte("t"))
	hash = alg.SumHash().Bytes()
	checkBytes(t, input, expected, hash)
}

func checkBytes(t *testing.T, input, expected, result []byte) {
	if !bytes.Equal(expected, result) {
		t.Errorf("hash mismatch: expect: %s have: %s, input is %s", expected, result, input)
	} else {
		t.Logf("hash test ok: expect: %s, input: %s", expected, input)
	}
}

// SHA3_256 bench
func BenchmarkSha3_256(b *testing.B) {
	a := []byte("Bench me!")
	alg, _ := NewHashAlgo(SHA3_256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alg.ComputeBytesHash(a)
	}
	return
}

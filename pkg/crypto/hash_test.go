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

	alg, err := NewHasher(SHA3_256)
	if err != nil {
		log.Error(err.Error())
		return
	}
	hash := alg.ComputeHash(input)
	checkBytes(t, input, expected, hash)

	alg.Reset()
	alg.Add([]byte("te"))
	alg.Add([]byte("s"))
	alg.Add([]byte("t"))
	hash = alg.SumHash()
	checkBytes(t, input, expected, hash)
}

// Sanity checks of SHA3_384
func TestSha3_384(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("e516dabb23b6e30026863543282780a3ae0dccf05551cf0295178d7ff0f1b41eecb9db3ff219007c4e097260d58621bd")

	alg, err := NewHasher(SHA3_384)
	if err != nil {
		log.Error(err.Error())
		return
	}
	hash := alg.ComputeHash(input)
	checkBytes(t, input, expected, hash)

	alg.Reset()
	alg.Add([]byte("te"))
	alg.Add([]byte("s"))
	alg.Add([]byte("t"))
	hash = alg.SumHash()
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
	alg, _ := NewHasher(SHA3_256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alg.ComputeHash(a)
	}
	return
}

// SHA3_384 bench
func BenchmarkSha3_384(b *testing.B) {
	a := []byte("Bench me!")
	alg, _ := NewHasher(SHA3_384)
	for i := 0; i < b.N; i++ {
		alg.ComputeHash(a)
	}
	return
}

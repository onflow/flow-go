package crypto

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"golang.org/x/crypto/sha3"
)

// These tests are sanity checks.
// They should ensure that we don't e.g. use Sha3-224 instead of Sha3-256
// and that the sha3 library uses keccak-f permutation.
func TestSha3_256(t *testing.T) {
	fmt.Println("testing Sha3_256:")
	msg := []byte("test")
	exp, _ := hex.DecodeString("36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")

	checkhash(t, HashCompute, msg, exp)
}

func checkhash(t *testing.T, f func([]byte) Hash, msg, exp []byte) {
	sum := f(msg)
	if !bytes.Equal(exp, sum.HashToBytes()) {
		t.Fatalf("hash mismatch: want: %x have: %x, message is %x", exp, sum.HashToBytes(), msg)
	}
}

func BenchmarkSha3_256(b *testing.B) {
	a := []byte("Bench me!")
	for i := 0; i < b.N; i++ {
		HashCompute(a)
	}
	return
}

func BenchmarkSha3_256_withoutInit(b *testing.B) {
	a := []byte("Bench me!")
	h := sha3.New256()
	var digest [32]byte
	for i := 0; i < b.N; i++ {
		h.Write(a)
		h.Sum(digest[:0])
	}
	return
}

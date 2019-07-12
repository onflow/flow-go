package crypto

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"golang.org/x/crypto/sha3"
)

type testStruct struct {
	x string
	y string
}

func (struc *testStruct) Encode() []byte {
	return []byte(struc.x + struc.y)
}

// These tests are sanity checks.
func TestSha3_256(t *testing.T) {
	fmt.Println("testing Sha3_256:")
	input := []byte("test")
	expected, _ := hex.DecodeString("36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")
	hash := ComputeBytesHash(input).ToBytes()
	checkBytes(t, input, expected, hash)

	hash = ComputeStructHash(&testStruct{"te", "st"}).ToBytes()
	checkBytes(t, input, expected, hash)
}

func checkBytes(t *testing.T, input, expected, result []byte) {
	if !bytes.Equal(expected, result) {
		t.Fatalf("hash mismatch: expect: %x have: %x, input is %x", expected, result, input)
	}
}

func BenchmarkSha3_256(b *testing.B) {
	a := []byte("Bench me!")
	for i := 0; i < b.N; i++ {
		ComputeBytesHash(a)
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

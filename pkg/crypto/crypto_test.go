package crypto

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
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

	alg := InitHashAlgo("SHA3_256")
	hash := alg.ComputeBytesHash(input).ToBytes()
	checkBytes(t, input, expected, hash)

	hash = alg.ComputeStructHash(&testStruct{"te", "st"}).ToBytes()
	checkBytes(t, input, expected, hash)

	alg.Reset()
	alg.AddBytes([]byte("te"))
	alg.AddBytes([]byte("s"))
	alg.AddBytes([]byte("t"))
	hash = alg.SumHash().ToBytes()
	checkBytes(t, input, expected, hash)
}

func checkBytes(t *testing.T, input, expected, result []byte) {
	if !bytes.Equal(expected, result) {
		t.Errorf("hash mismatch: expect: %x have: %x, input is %x", expected, result, input)
	} else {
		t.Logf("hash test ok: expect: %x, input: %x", expected, input)
	}
}

func BenchmarkSha3_256(b *testing.B) {
	a := []byte("Bench me!")
	alg := InitHashAlgo("SHA3_256")
	for i := 0; i < b.N; i++ {
		alg.ComputeBytesHash(a)
	}
	return
}

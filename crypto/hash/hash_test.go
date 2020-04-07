package hash

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func checkBytes(t *testing.T, input, expected, result []byte) {
	if !bytes.Equal(expected, result) {
		t.Errorf("hash mismatch: expect: %x have: %x, input is %x", expected, result, input)
	} else {
		t.Logf("hash test ok: expect: %x, input: %x", expected, input)
	}
}

// Sanity checks of SHA3_256
func TestSha3_256(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")

	alg := NewSHA3_256()
	hash := alg.ComputeHash(input)
	checkBytes(t, input, expected, hash)

	alg.Reset()
	_, _ = alg.Write([]byte("te"))
	_, _ = alg.Write([]byte("s"))
	_, _ = alg.Write([]byte("t"))
	hash = alg.SumHash()
	checkBytes(t, input, expected, hash)
}

// Sanity checks of SHA3_384
func TestSha3_384(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("e516dabb23b6e30026863543282780a3ae0dccf05551cf0295178d7ff0f1b41eecb9db3ff219007c4e097260d58621bd")

	alg := NewSHA3_384()
	hash := alg.ComputeHash(input)
	checkBytes(t, input, expected, hash)

	alg.Reset()
	_, _ = alg.Write([]byte("te"))
	_, _ = alg.Write([]byte("s"))
	_, _ = alg.Write([]byte("t"))
	hash = alg.SumHash()
	checkBytes(t, input, expected, hash)
}

// Sanity checks of SHA2_256
func TestSha2_256(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08")

	alg := NewSHA2_256()
	hash := alg.ComputeHash(input)
	checkBytes(t, input, expected, hash)

	alg.Reset()
	_, _ = alg.Write([]byte("te"))
	_, _ = alg.Write([]byte("s"))
	_, _ = alg.Write([]byte("t"))
	hash = alg.SumHash()
	checkBytes(t, input, expected, hash)
}

// Sanity checks of SHA2_256
func TestSha2_384(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("768412320f7b0aa5812fce428dc4706b3cae50e02a64caa16a782249bfe8efc4b7ef1ccb126255d196047dfedf17a0a9")

	alg := NewSHA2_384()
	hash := alg.ComputeHash(input)
	checkBytes(t, input, expected, hash)

	alg.Reset()
	_, _ = alg.Write([]byte("te"))
	_, _ = alg.Write([]byte("s"))
	_, _ = alg.Write([]byte("t"))
	hash = alg.SumHash()
	checkBytes(t, input, expected, hash)
}

// SHA3_256 bench
func BenchmarkSha3_256(b *testing.B) {
	a := []byte("Bench me!")
	alg := NewSHA3_256()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alg.ComputeHash(a)
	}
	return
}

// SHA3_384 bench
func BenchmarkSha3_384(b *testing.B) {
	a := []byte("Bench me!")
	alg := NewSHA3_384()
	for i := 0; i < b.N; i++ {
		alg.ComputeHash(a)
	}
	return
}

// SHA2_256 bench
func BenchmarkSha2_256(b *testing.B) {
	a := []byte("Bench me!")
	alg := NewSHA2_256()
	for i := 0; i < b.N; i++ {
		alg.ComputeHash(a)
	}
	return
}

// SHA2_256 bench
func BenchmarkSha2_384(b *testing.B) {
	a := []byte("Bench me!")
	alg := NewSHA2_384()
	for i := 0; i < b.N; i++ {
		alg.ComputeHash(a)
	}
	return
}

// Sanity checks of cSHAKE-128
// the test vector is taken from the NIST document
// https://csrc.nist.gov/CSRC/media/Projects/Cryptographic-Standards-and-Guidelines/documents/examples/Kmac_samples.pdf
func TestKmac128(t *testing.T) {

	input := []byte{0x00, 0x01, 0x02, 0x03}
	expected := [][]byte{
		{0xE5, 0x78, 0x0B, 0x0D, 0x3E, 0xA6, 0xF7, 0xD3, 0xA4, 0x29, 0xC5, 0x70, 0x6A, 0xA4, 0x3A, 0x00,
			0xFA, 0xDB, 0xD7, 0xD4, 0x96, 0x28, 0x83, 0x9E, 0x31, 0x87, 0x24, 0x3F, 0x45, 0x6E, 0xE1, 0x4E},
		{0x3B, 0x1F, 0xBA, 0x96, 0x3C, 0xD8, 0xB0, 0xB5, 0x9E, 0x8C, 0x1A, 0x6D, 0x71, 0x88, 0x8B, 0x71,
			0x43, 0x65, 0x1A, 0xF8, 0xBA, 0x0A, 0x70, 0x70, 0xC0, 0x97, 0x9E, 0x28, 0x11, 0x32, 0x4A, 0xA5},
	}
	key := []byte{0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4A, 0x4B, 0x4C, 0x4D, 0x4E, 0x4F,
		0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5A, 0x5B, 0x5C, 0x5D, 0x5E, 0x5F}
	customizers := [][]byte{
		[]byte(""),
		[]byte("My Tagged Application"),
	}
	outputSize := 32

	alg, _ := NewKMAC_128(key, customizers[0], outputSize)
	_, _ = alg.Write(input[0:2])
	_, _ = alg.Write(input[2:])
	hash := alg.SumHash()
	checkBytes(t, input, expected[0], hash)

	for i := 0; i < len(customizers); i++ {
		alg, _ = NewKMAC_128(key, customizers[i], outputSize)
		hash = alg.ComputeHash(input)
		checkBytes(t, input, expected[i], hash)
	}
}

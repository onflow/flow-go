package hash

import (
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"
)

// Sanity checks of SHA3_256
func TestSanitySha3_256(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")

	alg := NewSHA3_256()
	hash := alg.ComputeHash(input)
	assert.Equal(t, Hash(expected), hash)
}

// Sanity checks of SHA3_384
func TestSanitySha3_384(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("e516dabb23b6e30026863543282780a3ae0dccf05551cf0295178d7ff0f1b41eecb9db3ff219007c4e097260d58621bd")

	alg := NewSHA3_384()
	hash := alg.ComputeHash(input)
	assert.Equal(t, Hash(expected), hash)
}

// Sanity checks of SHA2_256
func TestSanitySha2_256(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08")

	alg := NewSHA2_256()
	hash := alg.ComputeHash(input)
	assert.Equal(t, Hash(expected), hash)
}

// Sanity checks of SHA2_256
func TestSanitySha2_384(t *testing.T) {
	input := []byte("test")
	expected, _ := hex.DecodeString("768412320f7b0aa5812fce428dc4706b3cae50e02a64caa16a782249bfe8efc4b7ef1ccb126255d196047dfedf17a0a9")

	alg := NewSHA2_384()
	hash := alg.ComputeHash(input)
	assert.Equal(t, Hash(expected), hash)
}

// Sanity checks of KMAC128
// the test vector is taken from the NIST document
// https://csrc.nist.gov/CSRC/media/Projects/Cryptographic-Standards-and-Guidelines/documents/examples/Kmac_samples.pdf
func TestSanityKmac128(t *testing.T) {

	input := []byte{0x00, 0x01, 0x02, 0x03}
	expected := []Hash{
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

	alg, err := NewKMAC_128(key, customizers[0], outputSize)
	require.Nil(t, err)
	_, _ = alg.Write(input[0:2])
	_, _ = alg.Write(input[2:])
	hash := alg.SumHash()
	assert.Equal(t, expected[0], hash)

	for i := 0; i < len(customizers); i++ {
		alg, err = NewKMAC_128(key, customizers[i], outputSize)
		require.Nil(t, err)
		hash = alg.ComputeHash(input)
		assert.Equal(t, expected[i], hash)
	}

	// test short key length
	_, err = NewKMAC_128(key[:15], customizers[0], outputSize)
	assert.Error(t, err)
}

// TestHashersAPI tests the expected definition of the hashers APIs
func TestHashersAPI(t *testing.T) {

	newKmac128 := func() Hasher {
		kmac, err := NewKMAC_128([]byte("test_key________"), []byte("test_custommizer"), 32)
		if err != nil {
			panic("new kmac hasher failed")
		}
		return kmac
	}

	newHasherFunctions := [](func() Hasher){
		NewSHA2_256,
		NewSHA2_384,
		NewSHA3_256,
		NewSHA3_384,
		newKmac128,
	}

	r := time.Now().UnixNano()
	rand.Seed(r)
	t.Logf("math rand seed is %d", r)
	data := make([]byte, 1801)
	rand.Read(data)

	for _, newFunction := range newHasherFunctions {
		// Reset should empty the state
		h := newFunction()
		expectedEmptyHash := h.SumHash()
		h.Write(data)
		h.Reset()
		emptyHash := h.SumHash()
		assert.Equal(t, expectedEmptyHash, emptyHash)

		// SumHash on an empty state is equal to compute hash with empty data
		emptyHash = h.ComputeHash(nil)
		assert.Equal(t, expectedEmptyHash, emptyHash)

		// successive writes of data are equivalent to compute hash
		// of the concatenated data
		h = newFunction()
		hash1 := h.ComputeHash(data)

		h.Reset()
		_, _ = h.Write(data[:355])
		_, _ = h.Write(data[355:902])
		_, _ = h.Write(data[902:])
		hash2 := h.SumHash()
		assert.Equal(t, hash1, hash2)

		// ComputeHash output does not depend on the hasher state
		h = newFunction()

		h.Write([]byte("dummy data"))
		hash1 = h.ComputeHash(data)
		assert.Equal(t, hash1, hash2)
	}
}

// TestSha3 is a specific test of SHA3-256 and SHA3-388.
// It compares the hashes of random data of different lengths to
// the output of standard Go sha3.
func TestSha3(t *testing.T) {
	r := time.Now().UnixNano()
	rand.Seed(r)
	t.Logf("math rand seed is %d", r)

	t.Run("SHA3_256", func(t *testing.T) {
		for i := 0; i < 5000; i++ {
			value := make([]byte, i)
			rand.Read(value)
			expected := sha3.Sum256(value)

			// test hash computation using the hasher
			hasher := NewSHA3_256()
			h := hasher.ComputeHash(value)
			assert.Equal(t, expected[:], []byte(h))

			// test hash computation using the light api
			ComputeSHA3_256(h, value)
			assert.Equal(t, expected[:], []byte(h))
		}
	})

	t.Run("SHA3_384", func(t *testing.T) {
		for i := 0; i < 5000; i++ {
			value := make([]byte, i)
			rand.Read(value)
			expected := sha3.Sum384(value)

			hasher := NewSHA3_384()
			h := hasher.ComputeHash(value)
			assert.Equal(t, expected[:], []byte(h))
		}
	})
}

// Benchmark of all hashers' ComputeHash function
func BenchmarkComputeHash(b *testing.B) {

	m := make([]byte, 64)
	rand.Read(m)

	b.Run("SHA2_256", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			alg := NewSHA2_256()
			_ = alg.ComputeHash(m)
		}
		b.StopTimer()
	})

	b.Run("SHA2_384", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			alg := NewSHA2_384()
			_ = alg.ComputeHash(m)
		}
		b.StopTimer()
	})

	b.Run("SHA3_256", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			alg := NewSHA3_256()
			alg.ComputeHash(m)
		}
		b.StopTimer()
	})

	b.Run("SHA3_256_light", func(b *testing.B) {
		h := make([]byte, HashLenSha3_256)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ComputeSHA3_256(h, m)
		}
		b.StopTimer()
	})

	b.Run("SHA3_384", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			alg := NewSHA3_384()
			_ = alg.ComputeHash(m)
		}
		b.StopTimer()
	})

	// KMAC128 with 128 bytes output
	b.Run("KMAC128_128", func(b *testing.B) {
		alg, _ := NewKMAC_128([]byte("bench_key________"), []byte("bench_custommizer"), 32)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = alg.ComputeHash(m)
		}
		b.StopTimer()
	})
}

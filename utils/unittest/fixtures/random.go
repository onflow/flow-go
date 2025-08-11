package fixtures

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// RandomGenerator provides random value generation with consistent randomness.
type RandomGenerator struct {
	rng *rand.Rand
}

// RandomBytes generates n random bytes.
func (g *RandomGenerator) RandomBytes(t testing.TB, n int) []byte {
	bytes := make([]byte, n)
	read, err := g.rng.Read(bytes)
	require.NoError(t, err)
	require.Equal(t, n, read, "expected to read %d bytes, got %d", n, read)
	return bytes
}

// RandomString generates a random string of the specified length.
func (g *RandomGenerator) RandomString(length uint) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range length {
		result[i] = charset[g.rng.Intn(len(charset))]
	}
	return string(result)
}

// Uint64InRange generates a random uint64 in the specified range [min, max].
func (g *RandomGenerator) Uint64InRange(min, max uint64) uint64 {
	if min > max {
		min, max = max, min
	}
	return min + uint64(g.rng.Intn(int(max)+1-int(min)))
}

// Uint32 generates a random uint32.
func (g *RandomGenerator) Uint32() uint32 {
	return g.rng.Uint32()
}

// Uint64 generates a random uint64.
func (g *RandomGenerator) Uint64() uint64 {
	return g.rng.Uint64()
}

// Int31 generates a random Int31.
func (g *RandomGenerator) Int31() int32 {
	return g.rng.Int31()
}

// Int63 generates a random Int63.
func (g *RandomGenerator) Int63() int64 {
	return g.rng.Int63()
}

// Intn generates a random int in the range [0, n).
func (g *RandomGenerator) Intn(n int) int {
	return g.rng.Intn(n)
}

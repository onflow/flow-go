package fixtures

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// RandomGenerator provides random value generation with consistent randomness.
type RandomGenerator struct {
	*rand.Rand
}

// RandomBytes generates n random bytes.
func (g *RandomGenerator) RandomBytes(t testing.TB, n int) []byte {
	bytes := make([]byte, n)
	read, err := g.Read(bytes)
	require.NoError(t, err)
	require.Equal(t, n, read, "expected to read %d bytes, got %d", n, read)
	return bytes
}

// RandomString generates a random string of the specified length.
func (g *RandomGenerator) RandomString(length uint) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range length {
		result[i] = charset[g.Intn(len(charset))]
	}
	return string(result)
}

// Uint64InRange generates a random uint64 in the specified range [min, max].
func (g *RandomGenerator) Uint64InRange(min, max uint64) uint64 {
	if min > max {
		min, max = max, min
	}
	return min + uint64(g.Intn(int(max)+1-int(min)))
}

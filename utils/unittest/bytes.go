package unittest

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// RandomByteSlice is a test helper that generates a cryptographically secure random byte slice of size n.
func RandomByteSlice(t *testing.T, n int) []byte {
	require.Greater(t, n, 0, "size should be positive")

	byteSlice := make([]byte, n)
	n, err := rand.Read(byteSlice)
	require.NoErrorf(t, err, "failed to generate random byte slice of size %d", n)
	require.Equalf(t, n, len(byteSlice), "failed to generate random byte slice of size %d", n)

	return byteSlice
}

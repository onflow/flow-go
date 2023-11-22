package unittest

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

// RandomStringFixture is a test helper that generates a cryptographically secure random string of size n.
func RandomStringFixture(t *testing.T, n int) string {
	require.Greater(t, n, 0, "size should be positive")

	// The base64 encoding uses 64 different characters to represent data in
	// strings, which makes it possible to represent 6 bits of data with each
	// character (as 2^6 is 64). This means that every 3 bytes (24 bits) of
	// input data will be represented by 4 characters (4 * 6 bits) in the
	// base64 encoding. Consequently, base64 encoding increases the size of
	// the data by approximately 1/3 compared to the original input data.
	//
	// 1. (n+3) / 4 - This calculates how many groups of 4 characters are needed
	//    in the base64 encoded output to represent at least 'n' characters.
	//    The +3 ensures rounding up, as integer division truncates the result.
	//
	// 2. ... * 3 - Each group of 4 base64 characters represents 3 bytes
	//    of input data. This multiplication calculates the number of bytes
	//    needed to produce the required length of the base64 string.
	byteSlice := make([]byte, (n+3)/4*3)
	n, err := rand.Read(byteSlice)
	require.NoError(t, err)
	require.Equal(t, n, len(byteSlice))

	encodedString := base64.URLEncoding.EncodeToString(byteSlice)
	return encodedString[:n]
}

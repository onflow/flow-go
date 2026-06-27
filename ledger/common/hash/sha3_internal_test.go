package hash

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_hash256Plus_matches_hash256plus256 checks that for a 32-byte p2 the two
// absorption routines produce the same digest. hash256Plus is the variable-length
// path (p1 is 256 bits, p2 is an arbitrary-length slice) while hash256plus256 is the
// fixed 256+256-bit fast path. When p2 is exactly 32 bytes both absorb the identical
// 512-bit input under the same SHA3-256 padding, so their outputs must be equal.
func Test_hash256Plus_matches_hash256plus256(t *testing.T) {
	var p1, p2 Hash
	_, err := rand.Read(p1[:])
	require.NoError(t, err)
	_, err = rand.Read(p2[:])
	require.NoError(t, err)

	h1 := (&state{}).hash256Plus(p1, p2[:])
	h2 := (&state{}).hash256plus256(p1, p2)

	assert.Equal(t, h1, h2)
}

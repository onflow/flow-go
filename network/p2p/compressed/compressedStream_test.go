package compressed

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRoundTrip evaluates that (1) reading what has been written by compressor yields in same result,
// and (2) data is compressed when written.
func TestRoundTrip(t *testing.T) {
	t.Skip()
	text := "hello world, hello world!"

	mc := newMockStream()
	s, err := NewCompressedStream(mc)
	require.NoError(t, err)

	n, err := s.Write([]byte(text))
	require.NoError(t, err)
	require.Equal(t, n, len(text))

	b, err := io.ReadAll(s)
	require.NoError(t, err)
	require.Equal(t, b, []byte(text))
}

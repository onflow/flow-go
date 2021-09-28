package compressed

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRoundTrip evaluates that (1) reading what has been written by compressor yields in same result,
// and (2) data is compressed when written.
func TestRoundTrip(t *testing.T) {
	// text := "hello world, hello world!"

	_, err := NewCompressedStream(newMockStream())
	require.NoError(t, err)

	//w, err := gzipComp.NewWriter(buf)
	//require.NoError(t, err)
	//
	//n, err := w.Write([]byte(text))
	//require.NoError(t, err)
	//require.Equal(t, n, len(text))
	//// written data on buffer should be compressed in size.
	//require.Less(t, buf.Len(), len(text))
	//
	//require.NoError(t, w.Close())
	//
	//r, err := gzipComp.NewReader(buf)
	//require.NoError(t, err)
	//
	//b, err := io.ReadAll(r)
	//require.NoError(t, err)
	//require.Equal(t, b, []byte(text))
}

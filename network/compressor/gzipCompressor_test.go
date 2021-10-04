package compressor_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/compressor"
)

// TestRoundTrip evaluates that (1) reading what has been written by compressor yields in same result,
// and (2) data is compressed when written.
func TestRoundTrip(t *testing.T) {
	text := "hello world, hello world!"
	textBytes := []byte(text)
	textBytesLen := len(textBytes)
	buf := new(bytes.Buffer)

	gzipComp := compressor.GzipStreamCompressor{}

	w, err := gzipComp.NewWriter(buf)
	require.NoError(t, err)

	// testing write
	//
	n, err := w.Write(textBytes)
	require.NoError(t, err)
	// written bytes should match original data
	require.Equal(t, n, textBytesLen)
	// written data on buffer should be compressed in size.
	require.Less(t, buf.Len(), textBytesLen)
	require.NoError(t, w.Close())

	// testing read
	//
	r, err := gzipComp.NewReader(buf)
	require.NoError(t, err)
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, b, textBytes)
}

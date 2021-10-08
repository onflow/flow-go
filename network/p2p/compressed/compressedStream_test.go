package compressed

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHappyPath evaluates reading from a compressed stream retrieves what originally has been written on it.
func TestHappyPath(t *testing.T) {
	text := "hello world, hello world!"
	textByte := []byte(text)
	textByteLen := len(textByte)

	// creates a pair of compressed streams
	mca, mcb := newCompressedStreamPair(t)

	// writes on stream mca
	writeWG := sync.WaitGroup{}
	writeWG.Add(1)
	go func() {
		defer writeWG.Done()

		n, err := mca.Write(textByte)
		require.NoError(t, err)

		require.Equal(t, n, len(text))
	}()

	// write on stream mca should be read on steam mcb
	readWG := sync.WaitGroup{}
	readWG.Add(1)
	go func() {
		defer readWG.Done()

		b := make([]byte, textByteLen)
		n, err := mcb.Read(b)
		require.NoError(t, err)

		require.Equal(t, n, textByteLen)
		require.Equal(t, b, textByte)
	}()

	unittest.RequireReturnsBefore(t, writeWG.Wait, 1*time.Second, "timeout for writing on stream")
	unittest.RequireReturnsBefore(t, readWG.Wait, 1*time.Second, "timeout for reading from stream")
}

// newStreamPair is a test helper that creates a pair of compressed streams a and b such that
// a reads what b writes and b reads what a writes.
func newStreamPair() (*mockStream, *mockStream) {
	ra, wb := io.Pipe()
	rb, wa := io.Pipe()

	sa := newMockStream(wa, ra)
	sb := newMockStream(wb, rb)

	return sa, sb
}

// newCompressedStreamPair is a test helper that creates a pair of compressed streams a and b such that
// a reads what b writes and b reads what a writes.
func newCompressedStreamPair(t *testing.T) (*compressedStream, *compressedStream) {
	sa, sb := newStreamPair()

	mca, err := NewCompressedStream(sa, compressor.GzipStreamCompressor{})
	require.NoError(t, err)

	mcb, err := NewCompressedStream(sb, compressor.GzipStreamCompressor{})
	require.NoError(t, err)

	return mca, mcb
}

package compressed

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestRoundTrip evaluates that (1) reading what has been written by compressor yields in same result,
// and (2) data is compressed when written.
func TestRoundTrip(t *testing.T) {
	text := "hello world, hello world!"
	byteLen := len([]byte(text))

	mca, _, mcb, _ := newCompressedStreamPair(t)

	writeWG := sync.WaitGroup{}
	writeWG.Add(1)
	go func() {
		defer writeWG.Done()

		n, err := mca.Write([]byte(text))
		require.NoError(t, err)

		require.Equal(t, n, len(text))
	}()

	readWG := sync.WaitGroup{}
	readWG.Add(1)
	go func() {
		defer readWG.Done()

		b := make([]byte, byteLen)
		n, err := mcb.Read(b)
		require.NoError(t, err)

		require.Equal(t, n, byteLen)
		require.Equal(t, b, []byte(text))
	}()

	unittest.RequireReturnsBefore(t, writeWG.Wait, 1*time.Second, "timeout for writing on stream")
	unittest.RequireReturnsBefore(t, readWG.Wait, 1*time.Second, "timeout for reading from stream")
}

func newStreamPair() (*mockStream, *mockStream) {
	ra, wb := io.Pipe()
	rb, wa := io.Pipe()

	sa := newMockStream(wa, ra)
	sb := newMockStream(wb, rb)

	return sa, sb
}

func newCompressedStreamPair(t *testing.T) (*compressedStream, *mockStream, *compressedStream, *mockStream) {
	sa, sb := newStreamPair()

	mca, err := NewCompressedStream(sa)
	require.NoError(t, err)

	mcb, err := NewCompressedStream(sb)
	require.NoError(t, err)

	return mca, sa, mcb, sb
}

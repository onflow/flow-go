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

	mc := newMockStream()
	s, err := NewCompressedStream(mc)
	require.NoError(t, err)

	writeWG := sync.WaitGroup{}
	writeWG.Add(1)
	go func() {
		defer writeWG.Done()
		n, err := s.Write([]byte(text))
		require.NoError(t, err)
		require.Equal(t, n, len(text))
	}()

	readWG := sync.WaitGroup{}
	readWG.Add(1)
	go func() {
		defer readWG.Done()
		b, err := io.ReadAll(s)
		require.NoError(t, err)
		require.Equal(t, b, []byte(text))
	}()

	unittest.RequireReturnsBefore(t, writeWG.Wait, 1*time.Second, "timeout for writing on stream")
	unittest.RequireReturnsBefore(t, readWG.Wait, 1*time.Second, "timeout for reading from stream")

}

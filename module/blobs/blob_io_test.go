package blobs

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyRead(t *testing.T) {
	blobCh := make(chan Blob)
	bcr := NewBlobChannelReader(blobCh)
	close(blobCh)
	var buf [1]byte
	n, err := bcr.Read(buf[:])
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, io.EOF)
}

func TestWriteClosedChannel(t *testing.T) {
	blobCh := make(chan Blob)
	bcw := NewBlobChannelWriter(blobCh, 4)
	require.NoError(t, bcw.Close())
	var buf [8]byte
	_, err := bcw.Write(buf[:])
	require.ErrorIs(t, err, ErrBlobChannelWriterClosed)
}

func TestEmptyWrite(t *testing.T) {
	blobCh := make(chan Blob)
	bcw := NewBlobChannelWriter(blobCh, 4)
	close(blobCh)
	assert.Panics(t, func() {
		var buf [8]byte
		_, _ = bcw.Write(buf[:])
	})
}

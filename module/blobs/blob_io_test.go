package blobs

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestEmptyWrite(t *testing.T) {
	blobCh := make(chan Blob)
	bcw := NewBlobChannelWriter(blobCh, 4)
	close(blobCh)
	assert.Panics(t, func() {
		var buf [8]byte
		_, _ = bcw.Write(buf[:])
	})
}

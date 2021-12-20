package blobs

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyRead(t *testing.T) {
	bcr, bw := OutgoingBlobChannel()
	require.NoError(t, bw.Close())
	var buf [1]byte
	n, err := bcr.Read(buf[:])
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, io.EOF)
}

func TestEmptyWrite(t *testing.T) {
	bcw, br := IncomingBlobChannel(1024)
	require.NoError(t, bcw.Close())
	_, err := br.Receive()
	assert.ErrorIs(t, err, ErrClosedBlobChannel)
}

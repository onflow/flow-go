package network

import (
	"io"
)

// Compressor offers compressing and decompressing services for sending and receiving
// a byte slice at network layer.
type Compressor interface {
	NewReader(io.Reader) (io.ReadCloser, error)
	NewWriter(io.Writer) (WriteCloseFlusher, error)
}

type WriteCloseFlusher interface {
	io.WriteCloser
	Flush() error
}

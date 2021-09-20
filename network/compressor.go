package network

import (
	"io"

	"github.com/libp2p/go-libp2p-core/network"
)

// Compressor offers compressing and decompressing services for sending and receiving
// a byte slice at network layer.
type Compressor interface {
	NewReader(stream network.Stream) (io.ReadCloser, error)
	NewWriter(stream network.Stream) (io.WriteCloser, error)
}

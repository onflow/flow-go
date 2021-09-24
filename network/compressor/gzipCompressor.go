package compressor

import (
	"compress/gzip"
	"io"

	"github.com/libp2p/go-libp2p-core/network"
)

type GzipStreamCompressor struct{}

func (g GzipStreamCompressor) NewReader(s network.Stream) (io.ReadCloser, error) {
	return gzip.NewReader(s)
}

func (g GzipStreamCompressor) NewWriter(s network.Stream) (io.WriteCloser, error) {
	return gzip.NewWriter(s), nil
}

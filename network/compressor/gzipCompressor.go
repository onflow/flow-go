package compressor

import (
	"compress/gzip"
	"io"

	"github.com/onflow/flow-go/network"
)

type GzipStreamCompressor struct{}

func (g GzipStreamCompressor) NewReader(r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}

func (g GzipStreamCompressor) NewWriter(w io.Writer) (network.WriteCloseFlusher, error) {
	return &gzipWriteCloseFlusher{w: gzip.NewWriter(w)}, nil
}

type gzipWriteCloseFlusher struct {
	w *gzip.Writer
}

func (g *gzipWriteCloseFlusher) Write(p []byte) (int, error) {
	return g.w.Write(p)
}

func (g *gzipWriteCloseFlusher) Close() error {
	return g.w.Close()
}

func (g *gzipWriteCloseFlusher) Flush() error {
	return g.w.Flush()
}

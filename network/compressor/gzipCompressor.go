package compressor

import (
	"compress/gzip"
	"io"

	"github.com/onflow/flow-go/network"
)

type GzipStreamCompressor struct{}

func (gzipStreamComp GzipStreamCompressor) NewReader(r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}

func (gzipStreamComp GzipStreamCompressor) NewWriter(w io.Writer) (network.WriteCloseFlusher, error) {
	return &gzipWriteCloseFlusher{w: gzip.NewWriter(w)}, nil
}

type gzipWriteCloseFlusher struct {
	w *gzip.Writer
}

func (gzipW *gzipWriteCloseFlusher) Write(p []byte) (int, error) {
	return gzipW.w.Write(p)
}

func (gzipW *gzipWriteCloseFlusher) Close() error {
	return gzipW.w.Close()
}

func (gzipW *gzipWriteCloseFlusher) Flush() error {
	return gzipW.w.Flush()
}

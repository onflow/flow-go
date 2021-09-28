package compressor

import (
	"compress/gzip"
	"io"
)

type GzipStreamCompressor struct{}

func (g GzipStreamCompressor) NewReader(r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}

func (g GzipStreamCompressor) NewWriter(w io.Writer) (io.WriteCloser, error) {
	return gzip.NewWriter(w), nil
}

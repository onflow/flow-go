package compressor

import (
	"io"

	"github.com/pierrec/lz4"

	"github.com/onflow/flow-go/network"
)

type Lz4Compressor struct{}

func NewLz4Compressor() *Lz4Compressor {
	return &Lz4Compressor{}
}

func (lz4Comp Lz4Compressor) NewReader(r io.Reader) (io.Reader, error) {
	return lz4.NewReader(r), nil
}

func (lz4Comp Lz4Compressor) NewWriter(w io.Writer) (network.WriteCloseFlusher, error) {
	return &lz4WriteCloseFlusher{w: lz4.NewWriter(w)}, nil
}

type lz4WriteCloseFlusher struct {
	w *lz4.Writer
}

func (lz4W *lz4WriteCloseFlusher) Write(p []byte) (int, error) {
	return lz4W.w.Write(p)
}

func (lz4W *lz4WriteCloseFlusher) Close() error {
	return lz4W.w.Close()
}

func (lz4W *lz4WriteCloseFlusher) Flush() error {
	return lz4W.w.Flush()
}

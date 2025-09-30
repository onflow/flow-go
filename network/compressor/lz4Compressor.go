package compressor

import (
	"io"

	// TODO: there was a bug in the v2.6 version that caused decompression to fail for some inputs on
	// intel CPUs. This is fixed on v4.1, however the newer version produces a different compressed
	// output. Using split versions for now to fix the compression issue. more investigation is needed
	// to understand why it's different. if we opt to upgrade the writer, it will need an HCU.
	lz4v2 "github.com/pierrec/lz4"
	lz4 "github.com/pierrec/lz4/v4"

	"github.com/onflow/flow-go/network"
)

var _ network.Compressor = (*Lz4Compressor)(nil)

type Lz4Compressor struct{}

func NewLz4Compressor() *Lz4Compressor {
	return &Lz4Compressor{}
}

func (lz4Comp Lz4Compressor) NewReader(r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(lz4.NewReader(r)), nil
}

func (lz4Comp Lz4Compressor) NewWriter(w io.Writer) (network.WriteCloseFlusher, error) {
	return &lz4WriteCloseFlusher{w: lz4v2.NewWriter(w)}, nil
}

type lz4WriteCloseFlusher struct {
	w *lz4v2.Writer
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

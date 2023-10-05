package lz4

import (
	"io"
	"sync"

	"github.com/pierrec/lz4"
	"google.golang.org/grpc/encoding"
)

// Name is the name registered for the LZ4 compressor.
const Name = "lz4"

func init() {
	c := &compressor{}
	c.poolCompressor.New = func() interface{} {
		return &writer{Writer: lz4.NewWriter(nil), pool: &c.poolCompressor}
	}
	encoding.RegisterCompressor(c)
}

type writer struct {
	*lz4.Writer
	pool *sync.Pool
}

func (c *compressor) Compress(w io.Writer) (io.WriteCloser, error) {
	z := c.poolCompressor.Get().(*writer)
	z.Reset(w)
	return z, nil
}

func (w *writer) Close() error {
	defer w.pool.Put(w)
	return w.Writer.Close()
}

type reader struct {
	*lz4.Reader
	pool *sync.Pool
}

func (c *compressor) Decompress(r io.Reader) (io.Reader, error) {
	z, inPool := c.poolDecompressor.Get().(*reader)
	if !inPool {
		newZ := lz4.NewReader(r)
		return &reader{Reader: newZ, pool: &c.poolDecompressor}, nil
	}
	z.Reset(r)
	return z, nil
}

func (r *reader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err == io.EOF {
		r.pool.Put(r)
	}
	return n, err
}

func (c *compressor) Name() string {
	return Name
}

type compressor struct {
	poolCompressor   sync.Pool
	poolDecompressor sync.Pool
}

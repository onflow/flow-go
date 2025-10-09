// Package deflate implements and registers the DEFLATE compressor
// during initialization.
package deflate

import (
	"io"
	"sync"

	"github.com/klauspost/compress/flate"
	"google.golang.org/grpc/encoding"
)

// Name is the name registered for the DEFLATE compressor.
const Name = "deflate"

func init() {
	c := &compressor{}
	w, _ := flate.NewWriter(nil, flate.DefaultCompression)
	c.poolCompressor.New = func() interface{} {
		return &writer{Writer: w, pool: &c.poolCompressor}
	}
	encoding.RegisterCompressor(c)
}

type compressor struct {
	poolCompressor   sync.Pool
	poolDecompressor sync.Pool
}

func (c *compressor) Name() string {
	return Name
}

func (c *compressor) Compress(w io.Writer) (io.WriteCloser, error) {
	dw := c.poolCompressor.Get().(*writer)
	dw.Reset(w)
	return dw, nil
}

func (c *compressor) Decompress(r io.Reader) (io.Reader, error) {
	dr, inPool := c.poolDecompressor.Get().(*reader)
	if !inPool {
		newR := flate.NewReader(r)
		return &reader{ReadCloser: newR, pool: &c.poolDecompressor}, nil
	}
	if err := dr.ReadCloser.(flate.Resetter).Reset(r, nil); err != nil {
		c.poolDecompressor.Put(dr)
		return nil, err
	}
	return dr, nil
}

type writer struct {
	*flate.Writer
	pool *sync.Pool
}

func (w *writer) Close() error {
	defer w.pool.Put(w)
	return w.Writer.Close()
}

type reader struct {
	io.ReadCloser
	pool *sync.Pool
}

func (r *reader) Read(p []byte) (n int, err error) {
	n, err = r.ReadCloser.Read(p)
	if err == io.EOF {
		r.pool.Put(r)
	}
	return n, err
}

package snappy

import (
	"io"
	"sync"

	"github.com/golang/snappy"
	"google.golang.org/grpc/encoding"
)

// Name is the name registered for the Snappy compressor.
const Name = "snappy"

func init() {
	c := &compressor{}
	c.poolCompressor.New = func() interface{} {
		return &writer{Writer: snappy.NewBufferedWriter(nil), pool: &c.poolCompressor}
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
	sw := c.poolCompressor.Get().(*writer)
	sw.Reset(w)
	return sw, nil
}

func (c *compressor) Decompress(r io.Reader) (io.Reader, error) {
	sr, inPool := c.poolDecompressor.Get().(*reader)
	if !inPool {
		return snappy.NewReader(r), nil
	}
	sr.Reset(r)
	return sr, nil
}

type writer struct {
	*snappy.Writer
	pool *sync.Pool
}

func (w *writer) Close() error {
	defer w.pool.Put(w)
	return w.Writer.Close()
}

type reader struct {
	*snappy.Reader
	pool *sync.Pool
}

func (r *reader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err == io.EOF {
		r.pool.Put(r)
	}
	return n, err
}

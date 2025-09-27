package zstd

import (
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
	"google.golang.org/grpc/encoding"
)

// Name is the name registered for the zstd compressor.
const Name = "zstd"

func init() {
	c := &compressor{}
	c.poolCompressor.New = func() any {
		newWriter, _ := zstd.NewWriter(io.Discard)
		return &writer{Encoder: newWriter, pool: &c.poolCompressor}
	}
	encoding.RegisterCompressor(c)
}

type writer struct {
	*zstd.Encoder
	pool *sync.Pool
}

// SetLevel updates the registered zstd compressor to use the compression level specified.
// NOTE: this function must only be called during initialization time (i.e. in an init() function),
// and is not thread-safe.
func SetLevel(level zstd.EncoderLevel) {
	c := encoding.GetCompressor(Name).(*compressor)
	c.poolCompressor.New = func() any {
		w, err := zstd.NewWriter(io.Discard, zstd.WithEncoderLevel(level))
		if err != nil {
			panic(err)
		}
		return &writer{Encoder: w, pool: &c.poolCompressor}
	}
}

func (c *compressor) Compress(w io.Writer) (io.WriteCloser, error) {
	z := c.poolCompressor.Get().(*writer)
	z.Encoder.Reset(w)
	return z, nil
}

func (z *writer) Close() error {
	defer z.pool.Put(z)
	return z.Encoder.Close()
}

type reader struct {
	*zstd.Decoder
	pool *sync.Pool
}

func (c *compressor) Decompress(r io.Reader) (io.Reader, error) {
	z, inPool := c.poolDecompressor.Get().(*reader)
	if !inPool {
		newZ, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return &reader{Decoder: newZ, pool: &c.poolDecompressor}, nil
	}
	if err := z.Reset(r); err != nil {
		c.poolDecompressor.Put(z)
		return nil, err
	}
	return z, nil
}

func (z *reader) Read(p []byte) (n int, err error) {
	n, err = z.Decoder.Read(p)
	if err == io.EOF {
		z.pool.Put(z)
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

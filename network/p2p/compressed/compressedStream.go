package compressed

import (
	"fmt"
	"io"
	"sync"

	"github.com/libp2p/go-libp2p-core/network"
	"go.uber.org/multierr"

	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/compressor"
)

// compressedStream is an internal networking layer data structure,
// which implements a compression mechanism as a wrapper around a native
// libp2p stream.
type compressedStream struct {
	network.Stream

	writeLock  sync.Mutex
	readLock   sync.Mutex
	compressor flownet.Compressor

	r io.ReadCloser
	w flownet.WriteCloseFlusher
}

type StreamOptFunc func(*compressedStream)

func WithStreamCompressor(comp flownet.Compressor) StreamOptFunc {
	return func(stream *compressedStream) {
		stream.compressor = comp
	}
}

func NewCompressedStream(s network.Stream, opts ...StreamOptFunc) (*compressedStream, error) {
	c := &compressedStream{
		Stream:     s,
		compressor: compressor.GzipStreamCompressor{},
	}

	for _, opt := range opts {
		opt(c)
	}

	w, err := c.compressor.NewWriter(s)
	if err != nil {
		return nil, fmt.Errorf("could not create compressor writer: %w", err)
	}

	c.w = w

	return c, nil
}

func (c *compressedStream) Write(b []byte) (int, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	n, err := c.w.Write(b)
	return n, multierr.Combine(err, c.w.Flush())
}

func (c *compressedStream) Read(b []byte) (int, error) {
	c.readLock.Lock()
	defer c.readLock.Unlock()

	if c.r == nil {
		r, err := c.compressor.NewReader(c.Stream)
		if err != nil {
			return 0, fmt.Errorf("could not create compressor reader: %w", err)
		}

		c.r = r
	}

	n, err := c.r.Read(b)
	if err != nil {
		c.r.Close()
	}
	return n, err
}

func (c *compressedStream) Close() error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	if err := c.w.Close(); err != nil {
		return err
	}
	if err := c.Stream.Close(); err != nil {
		return err
	}

	return nil
}

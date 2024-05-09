package internal

import (
	"fmt"
	"io"
	"sync"

	"github.com/libp2p/go-libp2p/core/network"
	"go.uber.org/multierr"

	flownet "github.com/onflow/flow-go/network"
)

// CompressedStream is an internal networking layer data structure,
// which implements a compression mechanism as a wrapper around a native
// libp2p stream.
type CompressedStream struct {
	network.Stream

	writeLock  sync.Mutex
	readLock   sync.Mutex
	compressor flownet.Compressor

	r io.ReadCloser
	w flownet.WriteCloseFlusher
}

// NewCompressedStream creates a compressed stream with gzip as default compressor.
func NewCompressedStream(s network.Stream, compressor flownet.Compressor) (*CompressedStream, error) {
	c := &CompressedStream{
		Stream:     s,
		compressor: compressor,
	}

	w, err := c.compressor.NewWriter(s)
	if err != nil {
		return nil, fmt.Errorf("could not create compressor writer: %w", err)
	}

	c.w = w

	return c, nil
}

func (c *CompressedStream) Write(b []byte) (int, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	n, err := c.w.Write(b)

	return n, multierr.Combine(err, c.w.Flush())
}

func (c *CompressedStream) Read(b []byte) (int, error) {
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

func (c *CompressedStream) Close() error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	return multierr.Combine(c.w.Close(), c.Stream.Close())
}

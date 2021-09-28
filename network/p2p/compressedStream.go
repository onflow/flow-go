package p2p

import (
	"fmt"
	"io"
	"sync"

	"github.com/libp2p/go-libp2p-core/network"
	"go.uber.org/multierr"

	flownet "github.com/onflow/flow-go/network"
)

type compressedStream struct {
	network.Stream

	writeLock  sync.Mutex
	compressor flownet.Compressor

	r io.ReadCloser
	w io.WriteCloser
}

func newCompressedStream(s network.Stream, compressor flownet.Compressor) (network.Stream, error) {
	w, err := compressor.NewWriter(s)
	if err != nil {
		return nil, fmt.Errorf("could not create compressor writer: %w", err)
	}

	c := &compressedStream{
		Stream:     s,
		w:          w,
		compressor: compressor,
	}

	return c, nil
}

func (c *compressedStream) Write(b []byte) (int, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	return c.w.Write(b)
}

func (c *compressedStream) Read(b []byte) (int, error) {
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
	return n, nil
}

func (c *compressedStream) Close() error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	return multierr.Combine(c.w.Close(), c.Stream.Close())
}

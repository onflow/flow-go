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

	writeLock sync.Mutex

	r io.ReadCloser
	w io.WriteCloser
}

func newCompressedStream(s network.Stream, compressor flownet.Compressor) (network.Stream, error) {
	r, err := compressor.NewReader(s)
	if err != nil {
		return nil, fmt.Errorf("could not create compressor reader: %w", err)
	}

	w, err := compressor.NewWriter(s)
	if err != nil {
		return nil, fmt.Errorf("could not create compressor writer: %w", err)
	}

	return &compressedStream{
		r: r,
		w: w,
	}, nil
}

func (c *compressedStream) Write(b []byte) (int, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	n, err := c.w.Write(b)
	if err != nil {
		return -1, fmt.Errorf("could not write on compressed writer: %w", err)
	}
	return n, nil
}

func (c *compressedStream) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	if err != nil {
		return -1, fmt.Errorf("could not write on compressed reader: %w", err)
	}
	return n, nil
}

func (c *compressedStream) Close() error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	return multierr.Combine(c.w.Close(), c.r.Close(), c.Stream.Close())
}

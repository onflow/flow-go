package compressor

import (
	"bufio"

	"github.com/libp2p/go-libp2p-core/network"
)

type mockStream struct {
	network.Stream
	r bufio.Reader
	w bufio.Writer
}

func newMockStream() *mockStream {
	return &mockStream{
		r: bufio.Reader{},
		w: bufio.Writer{},
	}
}

func (m *mockStream) Read(p []byte) (n int, err error) {
	return m.r.Read(p)
}

func (m *mockStream) Write(p []byte) (n int, err error) {
	return m.w.Write(p)
}

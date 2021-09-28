package compressed

import (
	"bufio"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
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

func (m *mockStream) Close() error {
	return nil
}

func (m *mockStream) CloseRead() error {
	return nil
}

func (m *mockStream) Reset() error {
	return nil
}

func (m *mockStream) SetDeadline(_ time.Time) error {
	return nil
}

func (m *mockStream) SetReadDeadline(_ time.Time) error {
	return nil
}

func (m *mockStream) SetWriteDeadline(_ time.Time) error {
	return nil
}

func (m *mockStream) ID() string {
	return ""
}

func (m *mockStream) Protocol() protocol.ID {
	return ""
}

func (m *mockStream) SetProtocol(id protocol.ID) {

}

func (m *mockStream) Stat() network.Stat {
	return network.Stat{}
}

func (m *mockStream) Conn() network.Conn {
	return nil
}

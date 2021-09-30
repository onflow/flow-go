package compressed

import (
	"fmt"
	"io"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

type mockStream struct {
	pw *io.PipeWriter
	pr *io.PipeReader
}

func newMockStream() *mockStream {
	pr, pw := io.Pipe()
	return &mockStream{
		pw: pw,
		pr: pr,
	}
}

func (m *mockStream) Read(p []byte) (int, error) {
	n, err := m.pr.Read(p)
	fmt.Println("read: ", n, p)
	return n, err
}

func (m *mockStream) Write(p []byte) (int, error) {
	go func() {
		_, _ = m.pw.Write(p)
	}()

	return len(p), nil
}

func (m *mockStream) Close() error {
	return multierror.Append(m.CloseRead(), m.CloseWrite())
}

func (m *mockStream) CloseRead() error {
	return m.pr.Close()
}

func (m *mockStream) CloseWrite() error {
	return m.pw.Close()
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

package compressed

import (
	"io"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

// mockStream is a mocked libp2p stream that is implemented as a pipe with a reader and writer.
// Whatever is written on the stream is written by the writer on the pipe, which in turn makes
// it available for read by the reader.
type mockStream struct {
	pw *io.PipeWriter
	pr *io.PipeReader
}

func newMockStream(pw *io.PipeWriter, pr *io.PipeReader) *mockStream {
	return &mockStream{
		pw: pw,
		pr: pr,
	}
}

func (m *mockStream) Read(p []byte) (int, error) {
	n, err := m.pr.Read(p)
	return n, err
}

func (m *mockStream) Write(p []byte) (int, error) {
	return m.pw.Write(p)
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

func (m *mockStream) SetProtocol(_ protocol.ID) {}

func (m *mockStream) Stat() network.Stat {
	return network.Stat{}
}

func (m *mockStream) Conn() network.Conn {
	return nil
}

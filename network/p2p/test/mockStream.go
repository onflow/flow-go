package p2ptest

import (
	"io"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// MockStream is a mocked libp2p stream that is implemented as a pipe with a reader and writer.
// Whatever is written on the stream is written by the writer on the pipe, which in turn makes
// it available for read by the reader.
type MockStream struct {
	pw *io.PipeWriter
	pr *io.PipeReader
}

func NewMockStream(pw *io.PipeWriter, pr *io.PipeReader) *MockStream {
	return &MockStream{
		pw: pw,
		pr: pr,
	}
}

func (m *MockStream) Read(p []byte) (int, error) {
	n, err := m.pr.Read(p)
	return n, err
}

func (m *MockStream) Write(p []byte) (int, error) {
	return m.pw.Write(p)
}

func (m *MockStream) Close() error {
	return multierror.Append(m.CloseRead(), m.CloseWrite())
}

func (m *MockStream) CloseRead() error {
	return m.pr.Close()
}

func (m *MockStream) CloseWrite() error {
	return m.pw.Close()
}

func (m *MockStream) Reset() error {
	return nil
}

func (m *MockStream) SetDeadline(_ time.Time) error {
	return nil
}

func (m *MockStream) SetReadDeadline(_ time.Time) error {
	return nil
}

func (m *MockStream) SetWriteDeadline(_ time.Time) error {
	return nil
}

func (m *MockStream) ID() string {
	return ""
}

func (m *MockStream) Protocol() protocol.ID {
	return ""
}

func (m *MockStream) SetProtocol(_ protocol.ID) error {
	return nil
}

func (m *MockStream) Stat() network.Stats {
	return network.Stats{}
}

func (m *MockStream) Conn() network.Conn {
	return nil
}

func (m *MockStream) Scope() network.StreamScope {
	return nil
}

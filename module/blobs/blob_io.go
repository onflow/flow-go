package blobs

import (
	"bytes"
	"errors"
	"io"

	"github.com/ipfs/go-cid"
)

var ErrBlobChannelWriterClosed = errors.New("blob channel writer is already closed")

// BlobChannelWriter is a writer which splits the data written to it into blobs and sends them to a blob channel.
type BlobChannelWriter struct {
	maxBlobSize int
	blobs       chan<- Blob
	buf         *bytes.Buffer
	cids        []cid.Cid
	closed      bool
	bytesSent   uint64
}

var _ io.WriteCloser = (*BlobChannelWriter)(nil)
var _ io.ByteWriter = (*BlobChannelWriter)(nil)

func (bw *BlobChannelWriter) BytesSent() uint64 {
	return bw.bytesSent
}

func (bw *BlobChannelWriter) CidsSent() []cid.Cid {
	return bw.cids
}

// Write writes len(data) bytes from data to the underlying blob channel. It returns the number of bytes written
// from data (0 <= n <= len(data)) or ErrBlobChannelWriterClosed if the Blob Channel Writer was already previously
// closed via a call to Close. It will always return a non-nil error if it returns n < len(data).
func (bw *BlobChannelWriter) Write(data []byte) (int, error) {
	if bw.closed {
		return 0, ErrBlobChannelWriterClosed
	}

	var n int
	for n < len(data) {
		size := bw.maxBlobSize - bw.buf.Len()
		if n+size > len(data) {
			size = len(data) - n
		}

		m, err := bw.buf.Write(data[n : n+size])
		n += m
		if err != nil {
			return n, err
		}

		// if we have a full blob, send it to the blob channel
		if bw.buf.Len() >= bw.maxBlobSize {
			bw.sendNewBlob()
		}
	}

	return n, nil
}

// sendNewBlob sends the currently buffered data to the blob channel and resets the buffer.
func (bw *BlobChannelWriter) sendNewBlob() {
	blob := NewBlob(bw.buf.Bytes())
	if bw.blobs != nil {
		bw.blobs <- blob
	}
	bw.cids = append(bw.cids, blob.Cid())
	bw.bytesSent += uint64(bw.buf.Len())

	// reset the buffer
	bw.buf = &bytes.Buffer{}
}

// WriteByte writes a single byte to the underlying blob channel. It returns an error if the byte could not be written.
// Returns ErrBlobChannelWriterClosed if the Blob Channel Writer was already previously closed via a call to Close
func (bw *BlobChannelWriter) WriteByte(c byte) error {
	if bw.closed {
		return ErrBlobChannelWriterClosed
	}

	if err := bw.buf.WriteByte(c); err != nil {
		return err
	}

	if bw.buf.Len() >= bw.maxBlobSize {
		bw.sendNewBlob()
	}

	return nil
}

// Flush flushes any buffered data to the underlying blob channel as a new blob. It returns an error if the flush failed.
// Returns ErrBlobChannelWriterClosed if the Blob Channel Writer was already previously closed via a call to Close
func (bw *BlobChannelWriter) Flush() error {
	if bw.closed {
		return ErrBlobChannelWriterClosed
	}

	if bw.buf.Len() > 0 {
		bw.sendNewBlob()
	}

	return nil
}

// Close flushes any buffered data to the underlying blob channel and closes the blob channel.
func (bw *BlobChannelWriter) Close() error {
	if err := bw.Flush(); err != nil {
		return err
	}

	bw.closed = true
	return nil
}

func NewBlobChannelWriter(blobChan chan<- Blob, maxBlobSize int) *BlobChannelWriter {
	return &BlobChannelWriter{
		maxBlobSize: maxBlobSize,
		blobs:       blobChan,
		buf:         &bytes.Buffer{},
	}
}

// BlobChannelReader is a reader which reads data from a blob channel.
type BlobChannelReader struct {
	blobs         <-chan Blob
	buf           *bytes.Buffer
	cids          []cid.Cid
	finished      bool
	bytesReceived uint64
}

var _ io.Reader = (*BlobChannelReader)(nil)
var _ io.ByteReader = (*BlobChannelReader)(nil)

func (br *BlobChannelReader) BytesReceived() uint64 {
	return br.bytesReceived
}

func (br *BlobChannelReader) CidsReceived() []cid.Cid {
	return br.cids
}

// Read reads up to len(data) bytes from the underlying blob channel into data. It returns the number of bytes read
// (0 <= n <= len(data)) and any error encountered.
// Returns io.EOF if the incoming blob channel was closed and all available data has been read.
func (br *BlobChannelReader) Read(data []byte) (int, error) {
	if br.finished {
		return 0, io.EOF
	}

	if len(data) == 0 {
		return 0, nil
	}

	// if all the data from the current buffer has been read, receive the next blob from the channel
	for br.buf.Len() == 0 {
		if !br.receiveNewBlob() {
			br.finished = true
			return 0, io.EOF
		}
	}

	return br.buf.Read(data)
}

// retrieveNewBlob retrieves a new blob from the blob channel and sets the buffer to the blob's data.
func (br *BlobChannelReader) receiveNewBlob() bool {
	blob, ok := <-br.blobs
	if !ok {
		return false
	}

	br.buf = bytes.NewBuffer(blob.RawData())
	br.cids = append(br.cids, blob.Cid())
	br.bytesReceived += uint64(len(blob.RawData()))

	return true
}

// ReadByte reads a single byte from the underlying blob channel. It returns an error if the byte could not be read.
// Returns io.EOF if the incoming blob channel was closed and all available data has been read.
func (br *BlobChannelReader) ReadByte() (byte, error) {
	if br.finished {
		return 0, io.EOF
	}

	// use a for loop here to guard against empty blobs
	for br.buf.Len() == 0 {
		if !br.receiveNewBlob() {
			br.finished = true
			return 0, io.EOF
		}
	}

	return br.buf.ReadByte()
}

func NewBlobChannelReader(blobChan <-chan Blob) *BlobChannelReader {
	return &BlobChannelReader{blobs: blobChan, buf: new(bytes.Buffer)}
}

package blobs

import (
	"errors"
	"io"
	"sync"
)

const defaultInitialBufCapacity = 1 << 17 // 128 KiB

var ErrClosedBlobChannel = errors.New("send/receive on closed blob channel")

// blobChannel represents a channel of blobs which can be closed without causing a panic
type blobChannel struct {
	blobs chan Blob
	once  sync.Once
	done  chan struct{}
}

// Send sends a blob to the blob channel. It returns ErrClosedBlobChannel if the channel is closed.
func (bc *blobChannel) Send(blob Blob) error {
	select {
	case <-bc.done:
		return ErrClosedBlobChannel
	default:
	}

	select {
	case bc.blobs <- blob:
		return nil
	case <-bc.done:
		return ErrClosedBlobChannel
	}
}

// Receives receives a blob from the blob channel. It returns ErrClosedBlobChannel if the channel is closed.
func (bc *blobChannel) Receive() (Blob, error) {
	select {
	case <-bc.done:
		return nil, ErrClosedBlobChannel
	default:
	}

	select {
	case blob := <-bc.blobs:
		return blob, nil
	case <-bc.done:
		return nil, ErrClosedBlobChannel
	}
}

// Close closes the blob channel. The returned error is always nil.
func (bc *blobChannel) Close() error {
	bc.once.Do(func() { close(bc.done) })
	return nil
}

// BlobChannelWriter is a writer which splits the data written to it into blobs and sends them to a blob channel.
type BlobChannelWriter struct {
	maxBlobSize int
	blobs       *blobChannel
	buf         []byte
	err         error
}

var _ io.WriteCloser = (*BlobChannelWriter)(nil)
var _ io.ByteWriter = (*BlobChannelWriter)(nil)

// Write writes len(data) bytes from data to the underlying blob channel. It returns the number of bytes written
// from data (0 <= n <= len(data)) and any error encountered that caused the write to stop early. It will always
// return a non-nil error if it returns n < len(data)
func (bw *BlobChannelWriter) Write(data []byte) (int, error) {
	var n, m int

	for n < len(data) && bw.err == nil {
		m, bw.err = bw.write(data[n:])
		n += m
	}

	return n, bw.err
}

// write writes some data from p to the underlying blob channel. It returns the number of bytes written from p
// (n <= len(p)), and any error encountered during the write.
func (bw *BlobChannelWriter) write(p []byte) (int, error) {
	n := bw.maxBlobSize - len(bw.buf)

	if n > len(p) {
		n = len(p)
	}

	bw.buf = append(bw.buf, p[:n]...)

	// if we have a full blob, send it to the blob channel
	if len(bw.buf) >= bw.maxBlobSize {
		return n, bw.sendNewBlob()
	}

	return n, nil
}

// sendNewBlob sends the currently buffered data to the blob channel and resets the buffer.
func (bw *BlobChannelWriter) sendNewBlob() error {
	blob := NewBlob(bw.buf)

	if err := bw.blobs.Send(blob); err != nil {
		return err
	}

	// reset the buffer
	bw.buf = make([]byte, 0, defaultInitialBufCapacity)

	return nil
}

// WriteByte writes a single byte to the underlying blob channel. It returns an error if the byte could not be written.
func (bw *BlobChannelWriter) WriteByte(c byte) error {
	if bw.err != nil {
		return bw.err
	}

	bw.buf = append(bw.buf, c)

	if len(bw.buf) >= bw.maxBlobSize {
		bw.err = bw.sendNewBlob()
	}

	return bw.err
}

// Flush flushes any buffered data to the underlying blob channel as a new blob. It returns an error if the flush failed.
func (bw *BlobChannelWriter) Flush() error {
	if bw.err != nil {
		return bw.err
	}

	if len(bw.buf) > 0 {
		bw.err = bw.sendNewBlob()
	}

	return bw.err
}

// Close flushes any buffered data to the underlying blob channel and closes the blob channel.
func (bw *BlobChannelWriter) Close() error {
	if err := bw.Flush(); err != nil {
		return err
	}

	bw.err = ErrClosedBlobChannel

	return bw.blobs.Close()
}

// BlobReceiver receives blobs from a blob channel.
type BlobReceiver struct {
	blobs *blobChannel
}

var _ io.Closer = (*BlobReceiver)(nil)

// Receive receives a blob from the blob channel. It returns ErrClosedBlobChannel if the channel is closed.
func (br *BlobReceiver) Receive() (Blob, error) {
	return br.blobs.Receive()
}

// Close closes the underlying blob channel.
func (br *BlobReceiver) Close() error {
	return br.blobs.Close()
}

// IncomingBlobChannel creates a BlobChannelWriter and BlobReceiver connected to the same underlying blob channel.
func IncomingBlobChannel(maxBlobSize int) (*BlobChannelWriter, *BlobReceiver) {
	blobChan := &blobChannel{
		blobs: make(chan Blob),
		done:  make(chan struct{}),
	}
	return &BlobChannelWriter{
		maxBlobSize: maxBlobSize,
		blobs:       blobChan,
		buf:         make([]byte, 0, defaultInitialBufCapacity),
	}, &BlobReceiver{blobChan}
}

// BlobChannelReader is a reader which reads data from a blob channel.
type BlobChannelReader struct {
	blobs *blobChannel
	buf   []byte
	err   error
}

var _ io.ReadCloser = (*BlobChannelReader)(nil)
var _ io.ByteReader = (*BlobChannelReader)(nil)

// Read reads up to len(data) bytes from the underlying blob channel into data. It returns the number of bytes read
// (0 <= n <= len(data)) and any error encountered. If some data is available but not len(data) bytes, Read will
// block until either enough data is available or an error occurs. This is in contrast to the conventional behavior
// which returns what is available instead of waiting for more.
func (br *BlobChannelReader) Read(data []byte) (int, error) {
	var n, m int

	for n < len(data) && br.err == nil {
		m, br.err = br.read(data[n:])
		n += m
	}

	return n, br.err
}

// read reades some data from the underlying blob channel into p. It returns the number of bytes read from p
// (n <= len(p)), and any error encountered during the read.
func (br *BlobChannelReader) read(p []byte) (int, error) {
	n := len(br.buf)

	// if all the data from the current buffer has been read, receive the next blob from the channel
	if n == 0 {
		err := br.receiveNewBlob()

		if errors.Is(err, ErrClosedBlobChannel) {
			return 0, io.EOF
		}

		return 0, err
	}

	if n > len(p) {
		n = len(p)
	}

	copy(p, br.buf[:n])
	br.buf = br.buf[n:]

	return n, nil
}

// retrieveNewBlob retrieves a new blob from the blob channel and sets the buffer to the blob's data.
func (br *BlobChannelReader) receiveNewBlob() error {
	blob, err := br.blobs.Receive()

	if err != nil {
		return err
	}

	br.buf = blob.RawData()

	return nil
}

// ReadByte reads a single byte from the underlying blob channel. It returns an error if the byte could not be read.
func (br *BlobChannelReader) ReadByte() (byte, error) {
	if br.err != nil {
		return 0, br.err
	}

	// use a for loop here to guard against empty blobs
	for len(br.buf) == 0 {
		if err := br.receiveNewBlob(); err != nil {
			if errors.Is(err, ErrClosedBlobChannel) {
				br.err = io.EOF
			} else {
				br.err = err
			}

			return 0, err
		}
	}

	b := br.buf[0]
	br.buf = br.buf[1:]

	return b, nil
}

// Close closes the underlying blob channel.
func (br *BlobChannelReader) Close() error {
	br.err = ErrClosedBlobChannel

	return br.blobs.Close()
}

// BlobSender sends blobs to a blob channel.
type BlobSender struct {
	blobs *blobChannel
}

var _ io.Closer = (*BlobSender)(nil)

// Send sends a blob to the blob channel. It returns ErrClosedBlobChannel if the channel is closed.
func (bs *BlobSender) Send(blob Blob) error {
	return bs.blobs.Send(blob)
}

// Close closes the underlying blob channel.
func (bs *BlobSender) Close() error {
	return bs.blobs.Close()
}

// OutgoingBlobChannel creates a BlobChannelReader and BlobSender connected to the same underlying blob channel.
func OutgoingBlobChannel() (*BlobChannelReader, *BlobSender) {
	blobChan := &blobChannel{
		blobs: make(chan Blob),
		done:  make(chan struct{}),
	}
	return &BlobChannelReader{blobs: blobChan}, &BlobSender{blobChan}
}

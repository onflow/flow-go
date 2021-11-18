package state_synchronization

import (
	"errors"
	"io"
	"sync"

	"github.com/onflow/flow-go/network"
)

var ErrClosedBlobChannel = errors.New("send/receive on closed blob channel")

type blobChannel struct {
	blobs chan network.Blob
	once  sync.Once
	done  chan struct{}
}

func (bc *blobChannel) Send(blob network.Blob) error {
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

func (bc *blobChannel) Receive() (network.Blob, error) {
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

func (bc *blobChannel) Close() error {
	bc.once.Do(func() { close(bc.done) })
	return nil
}

type BlobChannelWriter struct {
	maxBlobSize int
	blobs       *blobChannel
	buf         []byte
	err         error
}

var _ io.WriteCloser = (*BlobChannelWriter)(nil)
var _ io.ByteWriter = (*BlobChannelWriter)(nil)

func (bw *BlobChannelWriter) Write(data []byte) (int, error) {
	var n, m int

	for n < len(data) && bw.err == nil {
		m, bw.err = bw.write(data[n:])
		n += m
	}

	return n, bw.err
}

func (bw *BlobChannelWriter) write(p []byte) (int, error) {
	n := bw.maxBlobSize - len(bw.buf)

	if n > len(p) {
		n = len(p)
	}

	bw.buf = append(bw.buf, p[:n]...)

	if len(bw.buf) >= bw.maxBlobSize {
		return n, bw.sendNewBlob()
	}

	return n, nil
}

func (bw *BlobChannelWriter) sendNewBlob() error {
	blob := network.NewBlob(bw.buf)

	if err := bw.blobs.Send(blob); err != nil {
		return err
	}

	// reset the buffer
	bw.buf = make([]byte, 0, defaultInitialBufCapacity)

	return nil
}

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

func (bw *BlobChannelWriter) Flush() error {
	if bw.err != nil {
		return bw.err
	}

	if len(bw.buf) > 0 {
		bw.err = bw.sendNewBlob()
	}

	return bw.err
}

func (bw *BlobChannelWriter) Close() error {
	if err := bw.Flush(); err != nil {
		return err
	}

	return bw.blobs.Close()
}

type BlobReceiver struct {
	blobs *blobChannel
}

var _ io.Closer = (*BlobReceiver)(nil)

func (br *BlobReceiver) Receive() (network.Blob, error) {
	return br.blobs.Receive()
}

func (br *BlobReceiver) Close() error {
	return br.blobs.Close()
}

const defaultInitialBufCapacity = 1 << 17 // 128 KiB

func IncomingBlobChannel(maxBlobSize int) (*BlobChannelWriter, *BlobReceiver) {
	blobChan := &blobChannel{
		blobs: make(chan network.Blob),
		done:  make(chan struct{}),
	}
	return &BlobChannelWriter{
		maxBlobSize: maxBlobSize,
		blobs:       blobChan,
		buf:         make([]byte, 0, defaultInitialBufCapacity),
	}, &BlobReceiver{blobChan}
}

type BlobChannelReader struct {
	blobs *blobChannel
	buf   []byte
	err   error
}

var _ io.ReadCloser = (*BlobChannelReader)(nil)
var _ io.ByteReader = (*BlobChannelReader)(nil)

func (br *BlobChannelReader) Read(data []byte) (int, error) {
	var n, m int

	for n < len(data) && br.err == nil {
		m, br.err = br.read(data[n:])
		n += m
	}

	return n, br.err
}

func (br *BlobChannelReader) read(p []byte) (int, error) {
	n := len(br.buf)

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

func (br *BlobChannelReader) receiveNewBlob() error {
	blob, err := br.blobs.Receive()

	if err != nil {
		return err
	}

	br.buf = blob.RawData()

	return nil
}

func (br *BlobChannelReader) ReadByte() (byte, error) {
	if br.err != nil {
		return 0, br.err
	}

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

func (br *BlobChannelReader) Close() error {
	br.err = ErrClosedBlobChannel

	return br.blobs.Close()
}

type BlobSender struct {
	blobs *blobChannel
}

var _ io.Closer = (*BlobSender)(nil)

func (bs *BlobSender) Send(blob network.Blob) error {
	return bs.blobs.Send(blob)
}

func (bs *BlobSender) Close() error {
	return bs.blobs.Close()
}

func OutgoingBlobChannel() (*BlobChannelReader, *BlobSender) {
	blobChan := &blobChannel{
		blobs: make(chan network.Blob),
		done:  make(chan struct{}),
	}
	return &BlobChannelReader{blobs: blobChan}, &BlobSender{blobChan}
}

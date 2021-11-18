package state_synchronization

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/network"
)

const (
	defaultMaxBlobSize      = 1 << 20 // 1MiB
	defaultMaxBlobTreeDepth = 2       // prevents malicious CID from causing download of unbounded amounts of data
	defaultBlobBatchSize    = 16
)

var ErrBlobTreeDepthExceeded = errors.New("blob tree depth exceeded")

// ExecutionDataStorer handles storing and loading execution data from a blobservice
type ExecutionDataStorer struct {
	serializer  *serializer
	blobService network.BlobService
	maxBlobSize int
}

func NewExecutionDataStorer(
	codec encoding.Codec,
	compressor network.Compressor,
	blobService network.BlobService,
) *ExecutionDataStorer {
	return &ExecutionDataStorer{&serializer{codec, compressor}, blobService, defaultMaxBlobSize}
}

func (s *ExecutionDataStorer) receiveBlobs(parent context.Context, br *BlobReceiver) ([]cid.Cid, error) {
	defer br.Close()

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	var blobs []network.Blob
	var cids []cid.Cid

	for {
		blob, err := br.Receive()

		if err != nil {
			if errors.Is(err, ErrClosedBlobChannel) {
				break
			}

			return nil, err
		}

		blobs = append(blobs, blob)
		cids = append(cids, blob.Cid())

		if len(blobs) == defaultBlobBatchSize {
			if err := s.blobService.AddBlobs(ctx, blobs...); err != nil {
				return nil, fmt.Errorf("failed to add blobs to blobservice: %w", err)
			}

			blobs = nil
		}
	}

	return cids, nil
}

func (s *ExecutionDataStorer) addBlobs(ctx context.Context, v interface{}) ([]cid.Cid, error) {
	bcw, br := IncomingBlobChannel(s.maxBlobSize)

	done := make(chan struct{})
	var err error

	go func() {
		defer close(done)
		defer bcw.Close()

		if err = s.serializer.Serialize(bcw, v); err != nil {
			return
		}

		err = bcw.Flush()
	}()

	cids, recvErr := s.receiveBlobs(ctx, br)

	if recvErr != nil {
		return nil, recvErr
	}

	<-done

	return cids, err
}

// Add constructs a blob tree for the given ExecutionData and adds it to the blobservice, and then returns the root CID.
func (s *ExecutionDataStorer) Add(ctx context.Context, sd *ExecutionData) (cid.Cid, error) {
	cids, err := s.addBlobs(ctx, sd)

	if err != nil {
		return cid.Undef, fmt.Errorf("failed to add execution data blobs: %w", err)
	}

	for {
		if len(cids) == 1 {
			return cids[0], nil
		}

		if cids, err = s.addBlobs(ctx, cids); err != nil {
			return cid.Undef, fmt.Errorf("failed to add cid blobs: %w", err)
		}
	}
}

func (s *ExecutionDataStorer) sendBlobs(parent context.Context, bs *BlobSender, cids []cid.Cid) error {
	defer bs.Close()

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	blobChan := s.blobService.GetBlobs(ctx, cids...)
	cachedBlobs := make(map[cid.Cid]network.Blob)

	for _, c := range cids {
		var err error
		blob, ok := cachedBlobs[c]

		if ok {
			delete(cachedBlobs, c)
		} else {
			blob, err = s.findBlob(blobChan, c, cachedBlobs)

			if err != nil {
				_, ok := err.(*BlobNotFoundError)

				// the blob channel may be closed as a result of the context being canceled,
				// in which case we should return the context error.
				if ok && ctx.Err() != nil {
					return ctx.Err()
				}

				return err
			}
		}

		if err = bs.Send(blob); err != nil {
			return err
		}
	}

	return nil
}

func (s *ExecutionDataStorer) findBlob(
	blobChan <-chan network.Blob,
	target cid.Cid,
	cache map[cid.Cid]network.Blob,
) (network.Blob, error) {
	for blob := range blobChan {
		// check blob size
		blobSize := len(blob.RawData())
		if blobSize > s.maxBlobSize {
			return nil, &BlobSizeLimitExceededError{blob.Cid()}
		}

		if blob.Cid() == target {
			return blob, nil
		}

		cache[blob.Cid()] = blob
	}

	return nil, &BlobNotFoundError{target}
}

func (s *ExecutionDataStorer) getBlobs(ctx context.Context, cids []cid.Cid) (interface{}, error) {
	bcr, bs := OutgoingBlobChannel()

	done := make(chan struct{})
	var v interface{}
	var err error

	go func() {
		defer close(done)
		defer bcr.Close()

		v, err = s.serializer.Deserialize(bcr)
	}()

	sendErr := s.sendBlobs(ctx, bs, cids)

	if sendErr != nil && !errors.Is(sendErr, ErrClosedBlobChannel) {
		return nil, sendErr
	}

	<-done

	if err != nil {
		return nil, &MalformedDataError{err}
	} else if sendErr != nil {
		// if deserialization completed without consuming all blobs, then the data is malformed.
		return nil, &MalformedDataError{errors.New("deserialization didn't consume all data")}
	}

	return v, err
}

// Get gets the ExecutionData for the given root CID from the blobservice.
func (s *ExecutionDataStorer) Get(ctx context.Context, c cid.Cid) (*ExecutionData, error) {
	cids := []cid.Cid{c}

	for i := uint(0); i < defaultMaxBlobTreeDepth; i++ {
		v, err := s.getBlobs(ctx, cids)

		if err != nil {
			return nil, fmt.Errorf("failed to get level %v of blob tree: %w", i, err)
		}

		switch v := v.(type) {
		case *ExecutionData:
			return v, nil
		case *[]cid.Cid:
			cids = *v
		}
	}

	return nil, ErrBlobTreeDepthExceeded
}

// MalformedDataError is returned when malformed data is found at some level of the requested
// blob tree. It likely indicates that the tree was generated incorrectly, and hence the request
// should not be retried.
type MalformedDataError struct {
	err error
}

func (e *MalformedDataError) Error() string {
	return fmt.Sprintf("blob tree contains malformed data: %v", e.err)
}

func (e *MalformedDataError) Unwrap() error { return e.err }

type BlobSizeLimitExceededError struct {
	cid cid.Cid
}

func (e *BlobSizeLimitExceededError) Error() string {
	return fmt.Sprintf("blob %v exceeds maximum blob size", e.cid.String())
}

type BlobNotFoundError struct {
	cid cid.Cid
}

func (e *BlobNotFoundError) Error() string {
	return fmt.Sprintf("blob %v not found", e.cid.String())
}

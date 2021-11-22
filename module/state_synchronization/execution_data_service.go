package state_synchronization

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

const (
	defaultMaxBlobSize      = 1 << 20 // 1MiB
	defaultMaxBlobTreeDepth = 2       // prevents malicious CID from causing download of unbounded amounts of data
	defaultBlobBatchSize    = 16
)

var ErrBlobTreeDepthExceeded = errors.New("blob tree depth exceeded")

// ExecutionDataService handles adding and getting execution data from a blobservice
type ExecutionDataService struct {
	serializer  *serializer
	blobService network.BlobService
	maxBlobSize int
	metrics     module.ExecutionDataServiceMetrics
}

func NewExecutionDataService(
	codec encoding.Codec,
	compressor network.Compressor,
	blobService network.BlobService,
	metrics module.ExecutionDataServiceMetrics,
) *ExecutionDataService {
	return &ExecutionDataService{&serializer{codec, compressor}, blobService, defaultMaxBlobSize, metrics}
}

func (s *ExecutionDataService) storeBatch(ctx context.Context, br *BlobReceiver) ([]network.Blob, error) {
	var blobs []network.Blob
	var err error

	for i := 0; i < defaultBlobBatchSize; i++ {
		var blob network.Blob
		blob, err = br.Receive()

		if err != nil {
			break
		}

		blobs = append(blobs, blob)
	}

	return blobs, err
}

func (s *ExecutionDataService) storeBlobs(parent context.Context, br *BlobReceiver) ([]cid.Cid, error) {
	defer br.Close()

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	var cids []cid.Cid

	for {
		batch, recvErr := s.storeBatch(ctx, br)

		for _, blob := range batch {
			cids = append(cids, blob.Cid())
		}

		if err := s.blobService.AddBlobs(ctx, batch); err != nil {
			return nil, err
		}

		if recvErr != nil {
			if recvErr != ErrClosedBlobChannel {
				// this is an unexpected error, and should never occur
				return nil, recvErr
			}

			break
		}
	}

	return cids, nil
}

func (s *ExecutionDataService) addBlobs(ctx context.Context, v interface{}) ([]cid.Cid, error) {
	bcw, br := IncomingBlobChannel(s.maxBlobSize)

	done := make(chan struct{})
	var serializeErr error

	go func() {
		defer close(done)
		defer bcw.Close()

		if serializeErr = s.serializer.Serialize(bcw, v); serializeErr != nil {
			return
		}

		serializeErr = bcw.Flush()
	}()

	cids, recvErr := s.storeBlobs(ctx, br)

	<-done

	if recvErr != nil {
		return nil, recvErr
	}

	return cids, serializeErr
}

// Add constructs a blob tree for the given ExecutionData and adds it to the blobservice, and then returns the root CID.
func (s *ExecutionDataService) Add(ctx context.Context, sd *ExecutionData) (cid.Cid, error) {
	s.metrics.ExecutionDataAddStarted()

	start := time.Now()
	cids, err := s.addBlobs(ctx, sd)

	if err != nil {
		s.metrics.ExecutionDataAddFinished(time.Since(start), false, 0)

		return cid.Undef, fmt.Errorf("failed to add execution data blobs: %w", err)
	}

	var blobTreeNodes int

	for {
		blobTreeNodes += len(cids)

		if len(cids) == 1 {
			s.metrics.ExecutionDataAddFinished(time.Since(start), true, blobTreeNodes)

			return cids[0], nil
		}

		if cids, err = s.addBlobs(ctx, cids); err != nil {
			s.metrics.ExecutionDataAddFinished(time.Since(start), false, blobTreeNodes)

			return cid.Undef, fmt.Errorf("failed to add cid blobs: %w", err)
		}
	}
}

func (s *ExecutionDataService) retrieveBlobs(parent context.Context, bs *BlobSender, cids []cid.Cid) error {
	defer bs.Close()

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	blobChan := s.blobService.GetBlobs(ctx, cids)
	cachedBlobs := make(map[cid.Cid]network.Blob)

	for _, c := range cids {
		var err error
		blob, ok := cachedBlobs[c]

		if ok {
			delete(cachedBlobs, c)
		} else {
			blob, err = s.findBlob(blobChan, c, cachedBlobs)

			if err != nil {
				return err
			}

			if blob == nil {
				// the blob channel may be closed as a result of the context being canceled,
				// in which case we should return the context error.
				if ctx.Err() != nil {
					return ctx.Err()
				}

				return &BlobNotFoundError{c}
			}
		}

		if err = bs.Send(blob); err != nil {
			return err
		}
	}

	return nil
}

func (s *ExecutionDataService) findBlob(
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

	return nil, nil
}

func (s *ExecutionDataService) getBlobs(ctx context.Context, cids []cid.Cid) (interface{}, error) {
	bcr, bs := OutgoingBlobChannel()

	done := make(chan struct{})
	var v interface{}
	var deserializeErr error

	go func() {
		defer close(done)
		defer bcr.Close()

		v, deserializeErr = s.serializer.Deserialize(bcr)
	}()

	sendErr := s.retrieveBlobs(ctx, bs, cids)

	<-done

	if sendErr != nil && errors.Is(sendErr, ErrClosedBlobChannel) {
		return nil, sendErr
	}

	if deserializeErr != nil {
		return nil, &MalformedDataError{deserializeErr}
	}

	// TODO: deserialization succeeds even if the blob channel reader has still has unconsumed
	// data, meaning that a malicious actor could fill the blob tree with lots of unnecessary
	// data by appending it at the end of the serialized data for each level. Eventually, we
	// will need to implement validation logic on Verification nodes to slash this behavior.

	return v, nil
}

// Get gets the ExecutionData for the given root CID from the blobservice.
func (s *ExecutionDataService) Get(ctx context.Context, c cid.Cid) (*ExecutionData, error) {
	s.metrics.ExecutionDataGetStarted()

	start := time.Now()
	cids := []cid.Cid{c}

	var blobTreeNodes int

	for i := uint(0); i < defaultMaxBlobTreeDepth; i++ {
		v, err := s.getBlobs(ctx, cids)

		if err != nil {
			s.metrics.ExecutionDataGetFinished(time.Since(start), false, blobTreeNodes)

			return nil, fmt.Errorf("failed to get level %v of blob tree: %w", i, err)
		}

		blobTreeNodes += len(cids)

		switch v := v.(type) {
		case *ExecutionData:
			s.metrics.ExecutionDataGetFinished(time.Since(start), true, blobTreeNodes)

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
	return fmt.Sprintf("malformed data: %v", e.err)
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

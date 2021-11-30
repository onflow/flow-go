package state_synchronization

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/network"
)

const (
	defaultMaxBlobSize      = 1 << 20 // 1MiB
	defaultMaxBlobTreeDepth = 1       // prevents malicious CID from causing download of unbounded amounts of data
	defaultBlobBatchSize    = 16
)

var ErrBlobTreeDepthExceeded = errors.New("blob tree depth exceeded")

// ExecutionDataService handles adding and getting execution data from a blobservice
type ExecutionDataService struct {
	serializer  *serializer
	blobService network.BlobService
	maxBlobSize int
	metrics     module.ExecutionDataServiceMetrics
	logger      zerolog.Logger
}

func NewExecutionDataService(
	codec encoding.Codec,
	compressor network.Compressor,
	blobService network.BlobService,
	metrics module.ExecutionDataServiceMetrics,
	logger zerolog.Logger,
) *ExecutionDataService {
	return &ExecutionDataService{
		&serializer{codec, compressor},
		blobService,
		defaultMaxBlobSize,
		metrics,
		logger.With().Str("component", "execution_data_service").Logger(),
	}
}

// receiveBatch receives a batch of blobs from the given BlobReceiver, and returns them as a slice
func (s *ExecutionDataService) receiveBatch(br *blobs.BlobReceiver) ([]blobs.Blob, error) {
	var batch []blobs.Blob
	var err error

	for i := 0; i < defaultBlobBatchSize; i++ {
		var blob blobs.Blob

		blob, err = br.Receive()

		if err != nil {
			break
		}

		batch = append(batch, blob)
	}

	return batch, err
}

// storeBlobs receives blobs from the given BlobReceiver, and stores them to the blobservice
func (s *ExecutionDataService) storeBlobs(parent context.Context, br *blobs.BlobReceiver, logger zerolog.Logger) ([]cid.Cid, error) {
	defer br.Close()

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	var cids []cid.Cid

	for {
		batch, recvErr := s.receiveBatch(br) // retrieve one batch at a time
		batchCids := zerolog.Arr()

		for _, blob := range batch {
			cids = append(cids, blob.Cid())
			batchCids.Str(blob.Cid().String())
		}

		batchLogger := logger.With().Array("cids", batchCids).Logger()

		if err := s.blobService.AddBlobs(ctx, batch); err != nil {
			batchLogger.Debug().Err(err).Msg("failed to add batch to blobservice")

			return nil, err
		}

		batchLogger.Debug().Msg("added batch to blobservice")

		if recvErr != nil {
			if recvErr != blobs.ErrClosedBlobChannel {
				// this is an unexpected error, and should never occur
				return nil, recvErr
			}

			// the blob channel was closed, meaning that all blobs have been received
			break
		}
	}

	return cids, nil
}

// addBlobs serializes the given object, splits the serialized data into blobs, and adds them to the blobservice
//
// blobs are added in a batched streaming fashion, using a separate goroutine to serialize the object and send the
// blobs of serialized data over a blob channel to the main routine, which adds them in batches to the blobservice.
func (s *ExecutionDataService) addBlobs(ctx context.Context, v interface{}, logger zerolog.Logger) ([]cid.Cid, error) {
	bcw, br := blobs.IncomingBlobChannel(s.maxBlobSize)

	done := make(chan struct{})
	var serializeErr error

	// start serialization goroutine
	go func() {
		defer close(done)
		defer bcw.Close()

		if serializeErr = s.serializer.Serialize(bcw, v); serializeErr != nil {
			logger.Debug().Err(serializeErr).Msg("failed to serialize execution data")

			return
		}

		serializeErr = bcw.Flush()

		if serializeErr != nil {
			logger.Debug().Err(serializeErr).Msg("flushed execution data")
		}
	}()

	cids, storeErr := s.storeBlobs(ctx, br, logger)

	<-done

	if storeErr != nil {
		// the blob channel may be closed as a result of the context being canceled,
		// in which case we should return the context error.
		if errors.Is(storeErr, blobs.ErrClosedBlobChannel) && ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, storeErr
	}

	return cids, serializeErr
}

// Add constructs a blob tree for the given ExecutionData and adds it to the blobservice, and then returns the root CID.
func (s *ExecutionDataService) Add(ctx context.Context, sd *ExecutionData) (cid.Cid, error) {
	logger := s.logger.With().Str("block_id", sd.BlockID.String()).Logger()
	logger.Debug().Msg("adding execution data")

	s.metrics.ExecutionDataAddStarted()

	start := time.Now()
	cids, err := s.addBlobs(ctx, sd, logger)

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

		if cids, err = s.addBlobs(ctx, cids, logger); err != nil {
			s.metrics.ExecutionDataAddFinished(time.Since(start), false, blobTreeNodes)

			return cid.Undef, fmt.Errorf("failed to add cid blobs: %w", err)
		}
	}
}

// retrieveBlobs retrieves the blobs for the given CIDs from the blobservice, and sends them to the given BlobSender
func (s *ExecutionDataService) retrieveBlobs(parent context.Context, bs *blobs.BlobSender, cids []cid.Cid, logger zerolog.Logger) error {
	defer bs.Close()

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	blobChan := s.blobService.GetBlobs(ctx, cids) // initiate a batch request for the given CIDs
	cachedBlobs := make(map[cid.Cid]blobs.Blob)
	cidCounts := make(map[cid.Cid]int) // used to account for duplicate CIDs

	for _, c := range cids {
		cidCounts[c] += 1
	}

	for _, c := range cids {
		blob, ok := cachedBlobs[c]

		if !ok {
			var err error

			blob, err = s.findBlob(blobChan, c, cachedBlobs, logger)

			if err != nil {
				var blobNotFound *BlobNotFoundError

				// the blob channel may be closed as a result of the context being canceled,
				// in which case we should return the context error.
				if errors.As(err, &blobNotFound) && ctx.Err() != nil {
					return ctx.Err()
				}

				return err
			}
		}

		cidCounts[c] -= 1

		if cidCounts[c] == 0 {
			delete(cachedBlobs, c)
		}

		if err := bs.Send(blob); err != nil {
			return fmt.Errorf("failed to send blob %v to blob channel: %w", blob.Cid(), err)
		}
	}

	return nil
}

// findBlob retrieves blobs from the given channel, caching them along the way, until it either
// finds the target blob or exhausts the channel.
func (s *ExecutionDataService) findBlob(
	blobChan <-chan blobs.Blob,
	target cid.Cid,
	cache map[cid.Cid]blobs.Blob,
	logger zerolog.Logger,
) (blobs.Blob, error) {
	targetLogger := logger.With().Str("target_cid", target.String()).Logger()
	targetLogger.Debug().Msg("finding blob")

	// Note: blobs are returned as they are found, in no particular order
	for blob := range blobChan {
		targetLogger.Debug().Str("cid", blob.Cid().String()).Msg("received blob")

		// check blob size
		blobSize := len(blob.RawData())

		if blobSize > s.maxBlobSize {
			return nil, &BlobSizeLimitExceededError{blob.Cid()}
		}

		cache[blob.Cid()] = blob

		if blob.Cid() == target {
			return blob, nil
		}
	}

	targetLogger.Debug().Msg("blob not found")

	return nil, &BlobNotFoundError{target}
}

// getBlobs gets the given CIDs from the blobservice, reassembles the blobs, and deserializes the reassembled data into an object.
//
// blobs are fetched from the blobservice and sent over a blob channel as they arrive, to a separate goroutine, which performs the
// deserialization in a streaming fashion.
func (s *ExecutionDataService) getBlobs(ctx context.Context, cids []cid.Cid, logger zerolog.Logger) (interface{}, error) {
	bcr, bs := blobs.OutgoingBlobChannel()

	done := make(chan struct{})
	var v interface{}
	var deserializeErr error

	// start deserialization goroutine
	go func() {
		defer close(done)
		defer bcr.Close()

		if v, deserializeErr = s.serializer.Deserialize(bcr); deserializeErr != nil {
			logger.Debug().Err(deserializeErr).Msg("failed to deserialize execution data")
		}
	}()

	retrieveErr := s.retrieveBlobs(ctx, bs, cids, logger)

	<-done

	if retrieveErr != nil && !errors.Is(retrieveErr, blobs.ErrClosedBlobChannel) {
		return nil, retrieveErr
	}

	if deserializeErr != nil {
		return nil, &MalformedDataError{deserializeErr}
	}

	// TODO: deserialization succeeds even if the blob channel reader has still has unconsumed data, meaning that a malicious actor
	// could fill the blob tree with lots of unnecessary data by appending it at the end of the serialized data for each level.
	// It's possible that we could detect this and fail deserialization using something like the following:
	// https://github.com/onflow/flow-go/blob/bd5320719266b045ae2cac954f6a56e1e79560eb/engine/access/rest/handlers.go#L189-L193
	// Eventually, we will need to implement validation logic on Verification nodes to slash this behavior.

	return v, nil
}

// Get gets the ExecutionData for the given root CID from the blobservice.
func (s *ExecutionDataService) Get(ctx context.Context, rootCid cid.Cid) (*ExecutionData, error) {
	logger := s.logger.With().Str("cid", rootCid.String()).Logger()
	logger.Debug().Msg("getting execution data")

	s.metrics.ExecutionDataGetStarted()

	start := time.Now()
	cids := []cid.Cid{rootCid}

	var blobTreeNodes int

	for i := uint(0); i <= defaultMaxBlobTreeDepth; i++ {
		v, err := s.getBlobs(ctx, cids, logger)

		if err != nil {
			s.metrics.ExecutionDataGetFinished(time.Since(start), false, blobTreeNodes)

			return nil, fmt.Errorf("failed to get level %d of blob tree: %w", i, err)
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

// BlobSizeLimitExceededError is returned when a blob exceeds the maximum size allowed.
type BlobSizeLimitExceededError struct {
	cid cid.Cid
}

func (e *BlobSizeLimitExceededError) Error() string {
	return fmt.Sprintf("blob %v exceeds maximum blob size", e.cid.String())
}

// BlobNotFoundError is returned when the blobservice failed to find a blob.
type BlobNotFoundError struct {
	cid cid.Cid
}

func (e *BlobNotFoundError) Error() string {
	return fmt.Sprintf("blob %v not found", e.cid.String())
}

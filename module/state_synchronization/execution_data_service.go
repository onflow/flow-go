package state_synchronization

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
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

type BlobTree [][]cid.Cid

// ExecutionDataService handles adding/getting execution data to/from a blobservice
type ExecutionDataService interface {
	// Add constructs a blob tree for the given ExecutionData and
	// adds it to the blobservice, and then returns the root CID
	// and list of all CIDs.
	Add(ctx context.Context, sd *ExecutionData) (flow.Identifier, BlobTree, error)

	// Get gets the ExecutionData for the given root CID from the blobservice.
	Get(ctx context.Context, rootID flow.Identifier) (*ExecutionData, error)
}

type executionDataServiceImpl struct {
	serializer  *serializer
	blobService network.BlobService
	maxBlobSize int
	metrics     module.ExecutionDataServiceMetrics
	logger      zerolog.Logger
}

var _ ExecutionDataService = (*executionDataServiceImpl)(nil)

func NewExecutionDataService(
	codec encoding.Codec,
	compressor network.Compressor,
	blobService network.BlobService,
	metrics module.ExecutionDataServiceMetrics,
	logger zerolog.Logger,
) *executionDataServiceImpl {
	return &executionDataServiceImpl{
		&serializer{codec, compressor},
		blobService,
		defaultMaxBlobSize,
		metrics,
		logger.With().Str("component", "execution_data_service").Logger(),
	}
}

func (s *executionDataServiceImpl) Ready() <-chan struct{} {
	return s.blobService.Ready()
}

func (s *executionDataServiceImpl) Done() <-chan struct{} {
	return s.blobService.Done()
}

// receiveBatch receives a batch of blobs from the given BlobReceiver, and returns them as a slice
func (s *executionDataServiceImpl) receiveBatch(ctx context.Context, br *blobs.BlobReceiver) ([]blobs.Blob, error) {
	var batch []blobs.Blob

	for i := 0; i < defaultBlobBatchSize; i++ {
		blob, err := br.Receive()

		if err != nil {
			// the blob channel may be closed as a result of the context being
			// canceled, in which case we should return the context error
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			// the blob channel is closed, signal to the caller that no more
			// blobs can be received
			if i == 0 {
				return nil, err
			}

			break
		}

		batch = append(batch, blob)
	}

	return batch, nil
}

// storeBlobs receives blobs from the given BlobReceiver, and stores them to the blobservice
func (s *executionDataServiceImpl) storeBlobs(parent context.Context, br *blobs.BlobReceiver, logger zerolog.Logger) ([]cid.Cid, error) {
	defer br.Close()

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	var cids []cid.Cid

	for {
		batch, recvErr := s.receiveBatch(ctx, br) // retrieve one batch at a time

		if recvErr != nil {
			if errors.Is(recvErr, blobs.ErrClosedBlobChannel) {
				// all blobs have been received
				break
			}

			return nil, recvErr
		}

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
	}

	return cids, nil
}

// addBlobs serializes the given object, splits the serialized data into blobs, and adds them to the blobservice
//
// blobs are added in a batched streaming fashion, using a separate goroutine to serialize the object and send the
// blobs of serialized data over a blob channel to the main routine, which adds them in batches to the blobservice.
func (s *executionDataServiceImpl) addBlobs(ctx context.Context, v interface{}, logger zerolog.Logger) ([]cid.Cid, uint64, error) {
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
			return nil, 0, ctx.Err()
		}

		return nil, 0, storeErr
	}

	return cids, bcw.TotalBytesWritten(), serializeErr
}

// Add constructs a blob tree for the given ExecutionData and adds it to the blobservice, and then returns the root CID.
func (s *executionDataServiceImpl) Add(ctx context.Context, sd *ExecutionData) (flow.Identifier, BlobTree, error) {
	logger := s.logger.With().Str("block_id", sd.BlockID.String()).Logger()
	logger.Debug().Msg("adding execution data")

	s.metrics.ExecutionDataAddStarted()

	start := time.Now()
	cids, totalBytes, err := s.addBlobs(ctx, sd, logger)

	if err != nil {
		s.metrics.ExecutionDataAddFinished(time.Since(start), false, 0)

		return flow.ZeroID, nil, fmt.Errorf("failed to add execution data blobs: %w", err)
	}

	var blobTreeSize uint64
	var blobTree BlobTree

	for {
		cidArr := zerolog.Arr()

		for _, cid := range cids {
			cidArr.Str(cid.String())
		}

		logger.Debug().Array("cids", cidArr).Msg("added blobs")

		blobTree = append(blobTree, cids)
		blobTreeSize += totalBytes

		if len(cids) == 1 {
			s.metrics.ExecutionDataAddFinished(time.Since(start), true, blobTreeSize)

			root, err := flow.CidToId(cids[0])
			return root, blobTree, err
		}

		if cids, totalBytes, err = s.addBlobs(ctx, cids, logger); err != nil {
			s.metrics.ExecutionDataAddFinished(time.Since(start), false, blobTreeSize)

			return flow.ZeroID, nil, fmt.Errorf("failed to add cid blobs: %w", err)
		}
	}
}

// retrieveBlobs retrieves the blobs for the given CIDs from the blobservice, and sends them to the given BlobSender
// in the order specified by the CIDs.
func (s *executionDataServiceImpl) retrieveBlobs(parent context.Context, bs *blobs.BlobSender, cids []cid.Cid, logger zerolog.Logger) error {
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

			if blob, err = s.findBlob(ctx, blobChan, c, cachedBlobs, logger); err != nil {
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
func (s *executionDataServiceImpl) findBlob(
	ctx context.Context,
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

	// the blob channel may be closed as a result of the context being canceled,
	// in which case we should return the context error.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	targetLogger.Debug().Msg("blob not found")

	return nil, &BlobNotFoundError{target}
}

// getBlobs gets the given CIDs from the blobservice, reassembles the blobs, and deserializes the reassembled data into an object.
//
// blobs are fetched from the blobservice and sent over a blob channel as they arrive, to a separate goroutine, which performs the
// deserialization in a streaming fashion.
func (s *executionDataServiceImpl) getBlobs(ctx context.Context, cids []cid.Cid, logger zerolog.Logger) (interface{}, uint64, error) {
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
		return nil, 0, retrieveErr
	}

	if deserializeErr != nil {
		return nil, 0, &MalformedDataError{deserializeErr}
	}

	// TODO: deserialization succeeds even if the blob channel reader has still has unconsumed data, meaning that a malicious actor
	// could fill the blob tree with lots of unnecessary data by appending it at the end of the serialized data for each level.
	// It's possible that we could detect this and fail deserialization using something like the following:
	// https://github.com/onflow/flow-go/blob/bd5320719266b045ae2cac954f6a56e1e79560eb/engine/access/rest/handlers.go#L189-L193
	// Eventually, we will need to implement validation logic on Verification nodes to slash this behavior.

	return v, bcr.TotalBytesRead(), nil
}

// Get gets the ExecutionData for the given root CID from the blobservice.
// The returned error will be:
// - MalformedDataError if some level of the blob tree cannot be properly deserialized
// - BlobSizeLimitExceededError if any blob in the blob tree exceeds the maximum blob size
// - BlobNotFoundError if some CID in the blob tree could not be found from the blobservice
func (s *executionDataServiceImpl) Get(ctx context.Context, rootID flow.Identifier) (*ExecutionData, error) {
	rootCid := flow.IdToCid(rootID)

	logger := s.logger.With().Str("cid", rootCid.String()).Logger()
	logger.Debug().Msg("getting execution data")

	s.metrics.ExecutionDataGetStarted()

	start := time.Now()
	cids := []cid.Cid{rootCid}

	var blobTreeSize uint64

	for i := uint(0); i <= defaultMaxBlobTreeDepth; i++ {
		v, totalBytes, err := s.getBlobs(ctx, cids, logger)

		if err != nil {
			s.metrics.ExecutionDataGetFinished(time.Since(start), false, blobTreeSize)

			return nil, fmt.Errorf("failed to get level %d of blob tree: %w", i, err)
		}

		blobTreeSize += totalBytes

		switch v := v.(type) {
		case *ExecutionData:
			s.metrics.ExecutionDataGetFinished(time.Since(start), true, blobTreeSize)

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

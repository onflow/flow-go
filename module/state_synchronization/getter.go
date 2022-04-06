package state_synchronization

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/network"
)

const defaultMaxBlobTreeDepth = 1 // prevents malicious CID from causing download of unbounded amounts of data

var ErrBlobTreeDepthExceeded = errors.New("blob tree depth exceeded")

type ExecutionDataGetterOption func(*executionDataGetterImpl)

func WithMaxBlobSize(size int) ExecutionDataGetterOption {
	return func(s *executionDataGetterImpl) {
		s.maxBlobSize = size
	}
}

func WithMaxBlobTreeDepth(depth int) ExecutionDataGetterOption {
	return func(s *executionDataGetterImpl) {
		s.maxBlobTreeDepth = depth
	}
}

func WithStatusTrackerFactory(factory StatusTrackerFactory) ExecutionDataGetterOption {
	return func(s *executionDataGetterImpl) {
		s.statusTrackerFactory = factory
	}
}

func NewExecutionDataGetter(
	codec encoding.Codec,
	compressor network.Compressor,
	blobService network.BlobService,
	metrics module.ExecutionDataGetterMetrics,
	logger zerolog.Logger,
	opts ...ExecutionDataGetterOption,
) *executionDataGetterImpl {
	eds := &executionDataGetterImpl{
		&serializer{codec, compressor},
		blobService,
		DefaultMaxBlobSize,
		defaultMaxBlobTreeDepth,
		metrics,
		logger.With().Str("component", "execution_data_service").Logger(),
		&NoopStatusTrackerFactory{},
	}

	for _, opt := range opts {
		opt(eds)
	}

	return eds
}

var _ ExecutionDataGetter = (*executionDataGetterImpl)(nil)

type executionDataGetterImpl struct {
	serializer           *serializer
	blobService          network.BlobService
	maxBlobSize          int
	maxBlobTreeDepth     int
	metrics              module.ExecutionDataGetterMetrics
	logger               zerolog.Logger
	statusTrackerFactory StatusTrackerFactory
}

// retrieveBlobs retrieves the blobs for the given CIDs from the blobservice, and sends them to the given BlobSender
// in the order specified by the CIDs.
func (s *executionDataGetterImpl) retrieveBlobs(parent context.Context, bs *blobs.BlobSender, cids []cid.Cid, logger zerolog.Logger) error {
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
func (s *executionDataGetterImpl) findBlob(
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
func (s *executionDataGetterImpl) getBlobs(ctx context.Context, cids []cid.Cid, logger zerolog.Logger) (interface{}, uint64, error) {
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

func (s *executionDataGetterImpl) getExecutionDataRoot(
	ctx context.Context,
	blockID flow.Identifier,
	rootID flow.Identifier,
	blobGetter network.BlobGetter,
	statusTracker StatusTracker,
) (*ExecutionDataRoot, error) {
	if err := statusTracker.StartTransfer(); err != nil {
		return nil, fmt.Errorf("failed to track transfer start: %w", err)
	}

	blob, err := blobGetter.GetBlob(ctx, flow.IdToCid(rootID))
	if err != nil {
		return nil, fmt.Errorf("failed to get execution data root blob: %w", err)
	}

	v, err := s.serializer.Deserialize(bytes.NewBuffer(blob.RawData()))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize execution data root blob: %w", err)
	}

	edRoot, ok := v.(*ExecutionDataRoot)
	if !ok {
		return nil, fmt.Errorf("execution data root blob is not an ExecutionDataRoot")
	}

	if edRoot.BlockID != blockID {
		return nil, &MismatchedBlockIDError{blockID, edRoot.BlockID}
	}

	return edRoot, nil
}

func (s *executionDataGetterImpl) GetChunkExecutionDatas(
	ctx context.Context,
	blockID flow.Identifier,
	blockHeight uint64,
	rootID flow.Identifier,
) ([]*ChunkExecutionData, uint64, error) {
	logger := s.logger.With().Str("root_id", rootID.String()).Logger()
	logger.Debug().Msg("getting execution data")

	s.metrics.ExecutionDataGetStarted()

	success := false
	blobGetter := s.blobService.GetSession(ctx)
	totalSize := atomic.NewUint64(0)
	start := time.Now()

	defer func() {
		s.metrics.ExecutionDataGetFinished(time.Since(start), success, int(totalSize.Load()))
	}()

	statusTracker := s.statusTrackerFactory.GetStatusTracker(blockID, blockHeight, rootID)

	edRoot, err := s.getExecutionDataRoot(ctx, blockID, rootID, blobGetter, statusTracker)
	if err != nil {
		return nil, 0, err
	}

	g, gCtx := errgroup.WithContext(ctx)

	chunkExecutionDatas := make([]*ChunkExecutionData, len(edRoot.ChunkExecutionDataIDs))
	for i, chunkID := range edRoot.ChunkExecutionDataIDs {
		i := i
		chunkID := chunkID

		g.Go(func() error {
			ced, size, err := s.getChunkExecutionData(gCtx, chunkID, blobGetter, logger.With().
				Str("chunk_id", chunkID.String()).
				Int("chunk_index", i).
				Logger(),
				statusTracker,
			)

			if err != nil {
				return fmt.Errorf("failed to get chunk execution data at index %d: %w", i, err)
			}

			chunkExecutionDatas[i] = ced
			totalSize.Add(size)

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, 0, err
	}

	if latestIncorporatedHeight, err := statusTracker.FinishTransfer(); err != nil {
		return nil, 0, fmt.Errorf("failed to track transfer finish: %w", err)
	} else {
		success = true
		return chunkExecutionDatas, latestIncorporatedHeight, nil
	}
}

func (s *executionDataGetterImpl) getChunkExecutionData(
	ctx context.Context,
	chunkExecutionDataID cid.Cid,
	blobGetter network.BlobGetter,
	logger zerolog.Logger,
	statusTracker StatusTracker,
) (*ChunkExecutionData, uint64, error) {
	var blobTreeSize uint64

	cids := []cid.Cid{chunkExecutionDataID}

	for i := 0; i <= s.maxBlobTreeDepth; i++ {
		err := statusTracker.TrackBlobs(cids)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to track blobs: %w", err)
		}

		v, totalBytes, err := s.getBlobs(ctx, cids, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get level %d of blob tree: %w", i, err)
		}

		logger.Debug().Uint64("total_bytes", totalBytes).Int("level", i).Msg("got level of blob tree")

		blobTreeSize += totalBytes

		switch v := v.(type) {
		case *ChunkExecutionData:
			return v, blobTreeSize, nil
		case *[]cid.Cid:
			cids = *v
		}
	}

	return nil, 0, ErrBlobTreeDepthExceeded
}

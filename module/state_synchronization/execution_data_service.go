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

const (
	defaultMaxBlobSize      = 1 << 20 // 1MiB
	defaultMaxBlobTreeDepth = 1       // prevents malicious CID from causing download of unbounded amounts of data
	defaultBlobBatchSize    = 16
)

var ErrBlobTreeDepthExceeded = errors.New("blob tree depth exceeded")

// ExecutionDataService handles adding/getting execution data to/from a blobservice
type ExecutionDataService interface {
	// Add constructs a blob tree from the given list of ChunkExecutionData and
	// adds it to the blobservice, and then returns the root ID.
	Add(ctx context.Context, blockID flow.Identifier, blockHeight uint64, ceds []*ChunkExecutionData) (flow.Identifier, error)

	// Get gets the list of ChunkExecutionData for the given root ID from the blobservice.
	Get(ctx context.Context, blockID flow.Identifier, blockHeight uint64, rootID flow.Identifier) ([]*ChunkExecutionData, uint64, error)
}

type ExecutionDataServiceOption func(*executionDataServiceImpl)

func WithMaxBlobSize(size int) ExecutionDataServiceOption {
	return func(s *executionDataServiceImpl) {
		s.maxBlobSize = size
	}
}

func WithMaxBlobTreeDepth(depth int) ExecutionDataServiceOption {
	return func(s *executionDataServiceImpl) {
		s.maxBlobTreeDepth = depth
	}
}

func WithBlobBatchSize(batchSize int) ExecutionDataServiceOption {
	return func(s *executionDataServiceImpl) {
		s.blobBatchSize = batchSize
	}
}

func WithStatusTrackerFactory(factory StatusTrackerFactory) ExecutionDataServiceOption {
	return func(s *executionDataServiceImpl) {
		s.statusTrackerFactory = factory
	}
}

type executionDataServiceImpl struct {
	serializer           *serializer
	blobService          network.BlobService
	maxBlobSize          int
	maxBlobTreeDepth     int
	blobBatchSize        int
	metrics              module.ExecutionDataServiceMetrics
	logger               zerolog.Logger
	statusTrackerFactory StatusTrackerFactory
	cidCacherFactory     CIDCacherFactory
}

var _ ExecutionDataService = (*executionDataServiceImpl)(nil)

func NewExecutionDataService(
	codec encoding.Codec,
	compressor network.Compressor,
	blobService network.BlobService,
	metrics module.ExecutionDataServiceMetrics,
	logger zerolog.Logger,
	opts ...ExecutionDataServiceOption,
) *executionDataServiceImpl {
	eds := &executionDataServiceImpl{
		&serializer{codec, compressor},
		blobService,
		defaultMaxBlobSize,
		defaultMaxBlobTreeDepth,
		defaultBlobBatchSize,
		metrics,
		logger.With().Str("component", "execution_data_service").Logger(),
		&NoopStatusTrackerFactory{},
		nil, // TODO
	}

	for _, opt := range opts {
		opt(eds)
	}

	return eds
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

	for i := 0; i < s.blobBatchSize; i++ {
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

		err := s.blobService.AddBlobs(ctx, batch)

		if logger.GetLevel() >= zerolog.DebugLevel {
			batchCids := zerolog.Arr()

			for _, blob := range batch {
				cids = append(cids, blob.Cid())
				batchCids.Str(blob.Cid().String())
			}

			batchLogger := logger.With().Array("cids", batchCids).Logger()

			if err != nil {
				batchLogger.Debug().Err(err).Msg("failed to add batch to blobservice")
			} else {
				batchLogger.Debug().Msg("added batch to blobservice")
			}
		}

		if err != nil {
			return nil, err
		}
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

func (s *executionDataServiceImpl) addExecutionDataRoot(
	ctx context.Context,
	blockID flow.Identifier,
	blockHeight uint64,
	chunkExecutionDataIDs []cid.Cid,
	cidCacher CIDCacher,
) (flow.Identifier, error) {
	edRoot := &ExecutionDataRoot{
		BlockID:               blockID,
		ChunkExecutionDataIDs: chunkExecutionDataIDs,
	}

	buf := new(bytes.Buffer)
	if err := s.serializer.Serialize(buf, edRoot); err != nil {
		return flow.ZeroID, fmt.Errorf("failed to serialize execution data root: %w", err)
	}

	rootBlob := blobs.NewBlob(buf.Bytes())
	if err := s.blobService.AddBlob(ctx, rootBlob); err != nil {
		return flow.ZeroID, fmt.Errorf("failed to add execution data root to blobservice: %w", err)
	}

	cidCacher.InsertRootCID(rootBlob.Cid())

	return flow.CidToId(rootBlob.Cid())
}

func (s *executionDataServiceImpl) Add(ctx context.Context, blockID flow.Identifier, blockHeight uint64, ceds []*ChunkExecutionData) (flow.Identifier, error) {
	logger := s.logger.With().Str("block_id", blockID.String()).Logger()
	logger.Debug().Msg("adding execution data")

	s.metrics.ExecutionDataAddStarted()

	success := false
	totalSize := atomic.NewUint64(0)
	chunkExecutionDataIDs := make([]cid.Cid, len(ceds))
	start := time.Now()

	defer func() {
		s.metrics.ExecutionDataAddFinished(time.Since(start), success, totalSize.Load())
	}()

	cidCacher := s.cidCacherFactory.GetCIDCacher(blockID, blockHeight)
	g, gCtx := errgroup.WithContext(ctx)

	for i, ced := range ceds {
		i := i
		ced := ced

		g.Go(func() error {
			cedID, size, err := s.addChunkExecutionData(gCtx, i, ced, logger.With().Int("chunk_index", i).Logger(), cidCacher)

			if err != nil {
				return fmt.Errorf("failed to add chunk execution data at index %d: %w", i, err)
			}

			chunkExecutionDataIDs[i] = cedID
			totalSize.Add(size)

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return flow.ZeroID, err
	}

	if rootID, err := s.addExecutionDataRoot(ctx, blockID, blockHeight, chunkExecutionDataIDs, cidCacher); err != nil {
		return flow.ZeroID, err
	} else {
		success = true
		return rootID, nil
	}
}

func (s *executionDataServiceImpl) addChunkExecutionData(
	ctx context.Context,
	chunkIndex int,
	ced *ChunkExecutionData,
	logger zerolog.Logger,
	cidCacher CIDCacher,
) (cid.Cid, uint64, error) {
	cids, totalBytes, err := s.addBlobs(ctx, ced, logger)
	if err != nil {
		return cid.Undef, 0, fmt.Errorf("failed to add chunk execution data blobs: %w")
	}

	var blobTreeSize uint64
	var height int

	for {
		cidCacher.InsertBlobTreeLevel(chunkIndex, height, cids)

		if logger.GetLevel() >= zerolog.DebugLevel {
			cidArr := zerolog.Arr()

			for _, cid := range cids {
				cidArr.Str(cid.String())
			}

			logger.Debug().Array("cids", cidArr).Msg("added blobs")
		}

		blobTreeSize += totalBytes

		if len(cids) == 1 {
			return cids[0], totalBytes, err
		}

		if cids, totalBytes, err = s.addBlobs(ctx, cids, logger); err != nil {
			return cid.Undef, 0, fmt.Errorf("failed to add cid blobs: %w", err)
		}

		height++
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

func (s *executionDataServiceImpl) getExecutionDataRoot(
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

// Get gets the ExecutionData for the given root CID from the blobservice.
// The returned error will be:
// - MalformedDataError if some level of the blob tree cannot be properly deserialized
// - BlobSizeLimitExceededError if any blob in the blob tree exceeds the maximum blob size
// - BlobNotFoundError if some CID in the blob tree could not be found from the blobservice
func (s *executionDataServiceImpl) Get(
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
		s.metrics.ExecutionDataGetFinished(time.Since(start), success, totalSize.Load())
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

func (s *executionDataServiceImpl) getChunkExecutionData(
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

type MismatchedBlockIDError struct {
	expected flow.Identifier
	actual   flow.Identifier
}

func (e *MismatchedBlockIDError) Error() string {
	return fmt.Sprintf("execution data root has mismatched block ID:\n"+
		"expected: %q\n"+
		"actual  : %q", e.expected, e.actual)
}

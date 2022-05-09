package requester

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"
	"github.com/teivah/onecontext"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
)

// blobSizeLimitExceededError is returned when a blob exceeds the maximum size allowed.
type blobSizeLimitExceededError struct {
	cid cid.Cid
}

func (e *blobSizeLimitExceededError) Error() string {
	return fmt.Sprintf("blob %v exceeds maximum blob size", e.cid.String())
}

// mismatchedBlockIDError is returned when the block ID of the execution data does not
// match what was requested.
type mismatchedBlockIDError struct {
	expected flow.Identifier
	actual   flow.Identifier
}

func (e *mismatchedBlockIDError) Error() string {
	return fmt.Sprintf("execution data block ID %v does not match expected block ID %v", e.actual, e.expected)
}

type job struct {
	ctx             context.Context
	resultID        flow.Identifier
	executionDataID flow.Identifier
	blockID         flow.Identifier
	blockHeight     uint64
}

type handler struct {
	jobsIn  chan<- interface{}
	jobsOut <-chan interface{}

	fulfiller   *fulfiller
	storage     *tracker.Storage
	blobService network.BlobService
	serializer  *execution_data.Serializer

	maxBlobSize      int
	retryBaseTimeout time.Duration
	numWorkers       int

	logger  zerolog.Logger
	metrics module.ExecutionDataRequesterMetrics

	component.Component
}

func newHandler(
	fulfiller *fulfiller,
	storage *tracker.Storage,
	blobService network.BlobService,
	serializer *execution_data.Serializer,
	maxBlobSize int,
	retryBaseTimeout time.Duration,
	numWorkers int,
	logger zerolog.Logger,
	metrics module.ExecutionDataRequesterMetrics,
) *handler {
	jobsIn, jobsOut := util.UnboundedChannel()

	h := &handler{
		jobsIn:           jobsIn,
		jobsOut:          jobsOut,
		fulfiller:        fulfiller,
		storage:          storage,
		blobService:      blobService,
		serializer:       serializer,
		maxBlobSize:      maxBlobSize,
		retryBaseTimeout: retryBaseTimeout,
		numWorkers:       numWorkers,
		logger:           logger.With().Str("subcomponent", "handler").Logger(),
		metrics:          metrics,
	}

	cmb := component.NewComponentManagerBuilder()
	for i := 0; i < h.numWorkers; i++ {
		cmb.AddWorker(h.loop)
	}

	cm := cmb.Build()
	h.Component = cm

	return h
}

func (h *handler) submitJob(j *job) {
	h.jobsIn <- j
}

func (h *handler) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	if util.WaitClosed(ctx, h.fulfiller.Ready()) == nil && util.WaitClosed(ctx, h.blobService.Ready()) == nil {
		ready()
	} else {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case j := <-h.jobsOut:
			h.handle(ctx, j.(*job))
		}
	}
}

func (h *handler) isRetryable(err error) bool {
	var malformedDataErr *execution_data.MalformedDataError
	if errors.As(err, &malformedDataErr) {
		return false
	}

	var blobSizeLimitExceededErr *blobSizeLimitExceededError
	return !errors.As(err, &blobSizeLimitExceededErr)
}

func (h *handler) handle(parentCtx irrecoverable.SignalerContext, j *job) {
	getCtx, cancel := onecontext.Merge(parentCtx, j.ctx)
	defer cancel()

	logger := h.logger.With().
		Str("result_id", j.resultID.String()).
		Str("block_id", j.blockID.String()).
		Str("execution_data_id", j.executionDataID.String()).
		Logger()

	attempts := 0

	if err := retry.Fibonacci(getCtx, h.retryBaseTimeout, func(ctx context.Context) error {
		logger.Debug().Msg("attempting to get execution data")
		attempts++

		if util.CheckClosed(ctx.Done()) {
			return ctx.Err()
		}

		start := time.Now()
		jr := &jobResult{
			resultID:    j.resultID,
			blockHeight: j.blockHeight,
		}
		executionData, size, err := h.getExecutionData(getCtx, j)
		duration := time.Since(start)

		if err != nil {
			if util.CheckClosed(ctx.Done()) {
				return ctx.Err()
			}

			retryable := h.isRetryable(err)
			h.metrics.RequestFailed(duration, retryable)

			if retryable {
				logger.Err(err).Msg("failed to get execution data, will retry")
				return retry.RetryableError(err)
			}

			jr.err = err
			h.fulfiller.submitJobResult(jr)
			return err
		}

		h.metrics.RequestSucceeded(j.blockHeight, duration, size, attempts)

		if executionData.BlockID != j.blockID {
			err := &mismatchedBlockIDError{
				expected: j.blockID,
				actual:   executionData.BlockID,
			}
			jr.err = err
			h.fulfiller.submitJobResult(jr)
			return err
		}

		logger.Debug().Msg("submitting execution data to fulfiller")
		jr.executionData = executionData
		h.fulfiller.submitJobResult(jr)

		return nil
	}); err != nil {
		logger.Err(err).Msg("failed to get execution data")
	}
}

func (h *handler) getExecutionData(ctx context.Context, j *job) (*execution_data.BlockExecutionData, uint64, error) {
	var bed *execution_data.BlockExecutionData
	blobGetter := h.blobService.GetSession(ctx)
	totalSize := atomic.NewUint64(0)

	err := h.storage.Update(func(trackBlobs func(uint64, ...cid.Cid) error) error {
		edRoot, err := h.getExecutionDataRoot(ctx, j.blockHeight, j.executionDataID, blobGetter, trackBlobs)
		if err != nil {
			return fmt.Errorf("failed to get execution data root: %w", err)
		}

		g, gCtx := errgroup.WithContext(ctx)

		chunkExecutionDatas := make([]*execution_data.ChunkExecutionData, len(edRoot.ChunkExecutionDataIDs))
		for i, chunkDataID := range edRoot.ChunkExecutionDataIDs {
			i := i
			chunkDataID := chunkDataID

			g.Go(func() error {
				ced, size, err := h.getChunkExecutionData(
					gCtx,
					j.blockHeight,
					chunkDataID,
					blobGetter,
					trackBlobs,
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
			return err
		}

		bed = &execution_data.BlockExecutionData{
			BlockID:             edRoot.BlockID,
			ChunkExecutionDatas: chunkExecutionDatas,
		}

		return nil
	})

	return bed, totalSize.Load(), err
}

func (h *handler) getExecutionDataRoot(
	ctx context.Context,
	blockHeight uint64,
	rootID flow.Identifier,
	blobGetter network.BlobGetter,
	trackBlobs func(uint64, ...cid.Cid) error,
) (*execution_data.BlockExecutionDataRoot, error) {
	rootCid := flow.IdToCid(rootID)

	if err := trackBlobs(blockHeight, rootCid); err != nil {
		return nil, fmt.Errorf("failed to track root blob: %w", err)
	}

	blob, err := blobGetter.GetBlob(ctx, rootCid)
	if err != nil {
		// TODO: technically, we should check if the error is ErrNotFound
		// otherwise don't wrap it with BlobNotFoundError
		return nil, execution_data.NewBlobNotFoundError(rootCid)
	}

	v, err := h.serializer.Deserialize(bytes.NewBuffer(blob.RawData()))
	if err != nil {
		return nil, execution_data.NewMalformedDataError(err)
	}

	edRoot, ok := v.(*execution_data.BlockExecutionDataRoot)
	if !ok {
		return nil, execution_data.NewMalformedDataError(fmt.Errorf("execution data root blob does not deserialize to a BlockExecutionDataRoot"))
	}

	return edRoot, nil
}

func (h *handler) getChunkExecutionData(
	ctx context.Context,
	blockHeight uint64,
	chunkExecutionDataID cid.Cid,
	blobGetter network.BlobGetter,
	trackBlobs func(uint64, ...cid.Cid) error,
) (*execution_data.ChunkExecutionData, uint64, error) {
	cids := []cid.Cid{chunkExecutionDataID}

	for i := 0; ; i++ {
		if err := trackBlobs(blockHeight, cids...); err != nil {
			return nil, 0, fmt.Errorf("failed to track blobs for level %d of blob tree: %w", i, err)
		}

		v, size, err := h.getBlobs(ctx, cids)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get level %d of blob tree: %w", i, err)
		}

		switch v := v.(type) {
		case *execution_data.ChunkExecutionData:
			return v, size, nil
		case *[]cid.Cid:
			cids = *v
		default:
			return nil, 0, execution_data.NewMalformedDataError(fmt.Errorf("blob tree contains unexpected type %T at level %d", v, i))
		}
	}
}

// getBlobs gets the given CIDs from the blobservice, reassembles the blobs, and deserializes the reassembled data into an object.
func (h *handler) getBlobs(ctx context.Context, cids []cid.Cid) (interface{}, uint64, error) {
	blobCh, resultCh := h.retrieveBlobs(ctx, cids)
	bcr := blobs.NewBlobChannelReader(blobCh)
	v, deserializeErr := h.serializer.Deserialize(bcr)
	result := <-resultCh

	if result.err != nil {
		return nil, 0, result.err
	}

	if deserializeErr != nil {
		return nil, 0, execution_data.NewMalformedDataError(deserializeErr)
	}

	return v, result.totalSize, nil
}

type retrieveBlobsResult struct {
	err       error
	totalSize uint64
}

// retrieveBlobs retrieves the blobs for the given CIDs from the blobservice.
func (h *handler) retrieveBlobs(parent context.Context, cids []cid.Cid) (<-chan blobs.Blob, <-chan *retrieveBlobsResult) {
	blobsOut := make(chan blobs.Blob, len(cids))
	resultCh := make(chan *retrieveBlobsResult, 1)

	go func() {
		result := &retrieveBlobsResult{}
		ctx, cancel := context.WithCancel(parent)
		defer cancel()
		defer close(blobsOut)
		defer func() {
			resultCh <- result
			close(resultCh)
		}()

		blobChan := h.blobService.GetBlobs(ctx, cids) // initiate a batch request for the given CIDs
		cachedBlobs := make(map[cid.Cid]blobs.Blob)
		cidCounts := make(map[cid.Cid]int) // used to account for duplicate CIDs

		for _, c := range cids {
			cidCounts[c] += 1
		}

		for _, c := range cids {
			blob, ok := cachedBlobs[c]

			if !ok {
				var err error

				if blob, err = h.findBlob(blobChan, c, cachedBlobs); err != nil {
					// the blob channel may be closed as a result of the context being canceled,
					// in which case we should return the context error.
					if ctxErr := ctx.Err(); ctxErr != nil {
						result.err = ctxErr
					} else {
						result.err = err
					}

					return
				}
			}

			cidCounts[c] -= 1

			if cidCounts[c] == 0 {
				delete(cachedBlobs, c)
				delete(cidCounts, c)
			}

			result.totalSize += uint64(len(blob.RawData()))
			blobsOut <- blob
		}
	}()

	return blobsOut, resultCh
}

// findBlob retrieves blobs from the given channel, caching them along the way, until it either
// finds the target blob or exhausts the channel.
func (h *handler) findBlob(
	blobChan <-chan blobs.Blob,
	target cid.Cid,
	cache map[cid.Cid]blobs.Blob,
) (blobs.Blob, error) {
	// Note: blobs are returned as they are found, in no particular order
	for blob := range blobChan {
		// check blob size
		blobSize := len(blob.RawData())

		if blobSize > h.maxBlobSize {
			return nil, &blobSizeLimitExceededError{blob.Cid()}
		}

		cache[blob.Cid()] = blob

		if blob.Cid() == target {
			return blob, nil
		}
	}

	return nil, execution_data.NewBlobNotFoundError(target)
}

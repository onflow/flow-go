package state_synchronization

import (
	"bytes"
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

const defaultBlobBatchSize = 16

type ExecutionDataAdderOption func(*executionDataAdderImpl)

func WithBlobSizeLimit(size int) ExecutionDataAdderOption {
	return func(s *executionDataAdderImpl) {
		s.maxBlobSize = size
	}
}

func WithBlobBatchSize(batchSize int) ExecutionDataAdderOption {
	return func(s *executionDataAdderImpl) {
		s.blobBatchSize = batchSize
	}
}

func NewExecutionDataAdder(
	codec encoding.Codec,
	compressor network.Compressor,
	blobService network.BlobService,
	logger zerolog.Logger,
	metrics module.ExecutionDataAdderMetrics,
	opts ...ExecutionDataAdderOption,
) *executionDataAdderImpl {
	eda := &executionDataAdderImpl{
		logger.With().Str("component", "execution_data_adder").Logger(),
		metrics,
		DefaultMaxBlobSize,
		defaultBlobBatchSize,
		&serializer{codec, compressor},
		blobService,
	}

	for _, opt := range opts {
		opt(eda)
	}

	return eda
}

var _ ExecutionDataAdder = (*executionDataAdderImpl)(nil)

type executionDataAdderImpl struct {
	logger        zerolog.Logger
	metrics       module.ExecutionDataAdderMetrics
	maxBlobSize   int
	blobBatchSize int
	serializer    *serializer
	blobService   network.BlobService
}

func (s *executionDataAdderImpl) AddExecutionDataRoot(
	ctx context.Context,
	blockID flow.Identifier,
	chunkExecutionDataCIDs []cid.Cid,
) (flow.Identifier, error) {
	if s.logger.GetLevel() >= zerolog.DebugLevel {
		cidArr := zerolog.Arr()

		for _, cid := range chunkExecutionDataCIDs {
			cidArr.Str(cid.String())
		}

		s.logger.Debug().Str("block_id", blockID.String()).Array("cids", cidArr).Msg("adding execution data root")
	}

	s.metrics.ExecutionDataRootAddStarted()

	success := false
	var size int
	start := time.Now()

	defer func() {
		s.metrics.ExecutionDataRootAddFinished(time.Since(start), success, size)
	}()

	edRoot := &ExecutionDataRoot{
		BlockID:               blockID,
		ChunkExecutionDataIDs: chunkExecutionDataCIDs,
	}

	buf := new(bytes.Buffer)
	if err := s.serializer.Serialize(buf, edRoot); err != nil {
		return flow.ZeroID, fmt.Errorf("failed to serialize execution data root: %w", err)
	}

	rootBlob := blobs.NewBlob(buf.Bytes())
	if err := s.blobService.AddBlob(ctx, rootBlob); err != nil {
		return flow.ZeroID, fmt.Errorf("failed to add execution data root to blobservice: %w", err)
	}

	success = true
	size = buf.Len()

	return flow.CidToId(rootBlob.Cid())
}

func (s *executionDataAdderImpl) AddChunkExecutionData(
	ctx context.Context,
	blockID flow.Identifier,
	chunkIndex int,
	ced *ChunkExecutionData,
) (cid.Cid, error) {
	logger := s.logger.With().Str("block_id", blockID.String()).Int("chunk_index", chunkIndex).Logger()
	logger.Debug().Msg("adding chunk execution data")

	s.metrics.ChunkExecutionDataAddStarted()

	success := false
	var blobTreeSize int
	start := time.Now()

	defer func() {
		s.metrics.ChunkExecutionDataAddFinished(time.Since(start), success, blobTreeSize)
	}()

	cids, totalBytes, err := s.addBlobs(ctx, ced, logger)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to add chunk execution data blobs: %w")
	}

	for {
		if logger.GetLevel() >= zerolog.DebugLevel {
			cidArr := zerolog.Arr()

			for _, cid := range cids {
				cidArr.Str(cid.String())
			}

			logger.Debug().Array("cids", cidArr).Msg("added blobs")
		}

		blobTreeSize += totalBytes

		if len(cids) == 1 {
			success = true
			return cids[0], nil
		}

		if cids, totalBytes, err = s.addBlobs(ctx, cids, logger); err != nil {
			return cid.Undef, fmt.Errorf("failed to add cid blobs: %w", err)
		}
	}
}

// addBlobs serializes the given object, splits the serialized data into blobs, and adds them to the blobservice
//
// blobs are added in a batched streaming fashion, using a separate goroutine to serialize the object and send the
// blobs of serialized data over a blob channel to the main routine, which adds them in batches to the blobservice.
func (s *executionDataAdderImpl) addBlobs(ctx context.Context, v interface{}, logger zerolog.Logger) ([]cid.Cid, int, error) {
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

	return cids, int(bcw.TotalBytesWritten()), serializeErr
}

// storeBlobs receives blobs from the given BlobReceiver, and stores them to the blobservice
func (s *executionDataAdderImpl) storeBlobs(parent context.Context, br *blobs.BlobReceiver, logger zerolog.Logger) ([]cid.Cid, error) {
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

// receiveBatch receives a batch of blobs from the given BlobReceiver, and returns them as a slice
func (s *executionDataAdderImpl) receiveBatch(ctx context.Context, br *blobs.BlobReceiver) ([]blobs.Blob, error) {
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

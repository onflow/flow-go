package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/network"
)

type ProviderOption func(*ExecutionDataProvider)

func WithBlobSizeLimit(size int) ProviderOption {
	return func(p *ExecutionDataProvider) {
		p.maxBlobSize = size
	}
}

// Provider is used to provide execution data blobs over the network via a blob service.
type Provider interface {
	Provide(ctx context.Context, blockHeight uint64, executionData *execution_data.BlockExecutionData) (flow.Identifier, *flow.BlockExecutionDataRoot, error)
}

type ExecutionDataProvider struct {
	logger       zerolog.Logger
	metrics      module.ExecutionDataProviderMetrics
	maxBlobSize  int
	blobService  network.BlobService
	storage      tracker.Storage
	cidsProvider *ExecutionDataCIDProvider
}

var _ Provider = (*ExecutionDataProvider)(nil)

func NewProvider(
	logger zerolog.Logger,
	metrics module.ExecutionDataProviderMetrics,
	serializer execution_data.Serializer,
	blobService network.BlobService,
	storage tracker.Storage,
	opts ...ProviderOption,
) *ExecutionDataProvider {
	p := &ExecutionDataProvider{
		logger:       logger.With().Str("component", "execution_data_provider").Logger(),
		metrics:      metrics,
		maxBlobSize:  execution_data.DefaultMaxBlobSize,
		cidsProvider: NewExecutionDataCIDProvider(serializer),
		blobService:  blobService,
		storage:      storage,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *ExecutionDataProvider) storeBlobs(parent context.Context, blockHeight uint64, blobCh <-chan blobs.Blob) <-chan error {
	ch := make(chan error, 1)
	go func() {
		defer close(ch)

		start := time.Now()

		var blobs []blobs.Blob
		var cids []cid.Cid
		var totalSize uint64
		for blob := range blobCh {
			blobs = append(blobs, blob)
			cids = append(cids, blob.Cid())
			totalSize += uint64(len(blob.RawData()))
		}

		if p.logger.Debug().Enabled() {
			cidArr := zerolog.Arr()
			for _, cid := range cids {
				cidArr = cidArr.Str(cid.String())
			}
			p.logger.Debug().Array("cids", cidArr).Uint64("height", blockHeight).Msg("storing blobs")
		}

		err := p.storage.Update(func(trackBlobs tracker.TrackBlobsFn) error {
			ctx, cancel := context.WithCancel(parent)
			defer cancel()

			// track new blobs so that they can be pruned later
			if err := trackBlobs(blockHeight, cids...); err != nil {
				return fmt.Errorf("failed to track blobs: %w", err)
			}

			if err := p.blobService.AddBlobs(ctx, blobs); err != nil {
				return fmt.Errorf("failed to add blobs: %w", err)
			}

			return nil
		})
		duration := time.Since(start)

		if err != nil {
			ch <- err
			p.metrics.AddBlobsFailed()
		} else {
			p.metrics.AddBlobsSucceeded(duration, totalSize)
		}
	}()

	return ch
}

// Provide adds the block execution data for a newly executed (generally not sealed or finalized) block to the blob store for distribution using Bitswap.
// It computes and returns the root CID of the execution data blob tree.
// This function returns once the root CID has been computed, and all blobs are successfully stored
// in the Bitswap Blobstore.
func (p *ExecutionDataProvider) Provide(ctx context.Context, blockHeight uint64, executionData *execution_data.BlockExecutionData) (flow.Identifier, *flow.BlockExecutionDataRoot, error) {
	rootID, rootData, errCh, err := p.provide(ctx, blockHeight, executionData)
	storeErr, ok := <-errCh

	if err != nil {
		return flow.ZeroID, nil, err
	}

	if ok {
		return flow.ZeroID, nil, storeErr
	}

	if err = p.storage.SetFulfilledHeight(blockHeight); err != nil {
		return flow.ZeroID, nil, err
	}

	return rootID, rootData, nil
}

func (p *ExecutionDataProvider) provide(ctx context.Context, blockHeight uint64, executionData *execution_data.BlockExecutionData) (flow.Identifier, *flow.BlockExecutionDataRoot, <-chan error, error) {
	logger := p.logger.With().Uint64("height", blockHeight).Str("block_id", executionData.BlockID.String()).Logger()
	logger.Debug().Msg("providing execution data")

	start := time.Now()

	blobCh := make(chan blobs.Blob)
	defer close(blobCh)

	errCh := p.storeBlobs(ctx, blockHeight, blobCh)
	g := new(errgroup.Group)

	chunkDataIDs := make([]cid.Cid, len(executionData.ChunkExecutionDatas))
	for i, chunkExecutionData := range executionData.ChunkExecutionDatas {
		i := i
		chunkExecutionData := chunkExecutionData

		g.Go(func() error {
			logger.Debug().Int("chunk_index", i).Msg("adding chunk execution data")
			cedID, err := p.cidsProvider.addChunkExecutionData(chunkExecutionData, blobCh)
			if err != nil {
				return fmt.Errorf("failed to add chunk execution data at index %d: %w", i, err)
			}
			logger.Debug().Int("chunk_index", i).Str("chunk_execution_data_id", cedID.String()).Msg("chunk execution data added")

			chunkDataIDs[i] = cedID
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return flow.ZeroID, nil, errCh, err
	}

	edRoot := &flow.BlockExecutionDataRoot{
		BlockID:               executionData.BlockID,
		ChunkExecutionDataIDs: chunkDataIDs,
	}
	rootID, err := p.cidsProvider.addExecutionDataRoot(edRoot, blobCh)
	if err != nil {
		return flow.ZeroID, nil, errCh, fmt.Errorf("failed to add execution data root: %w", err)
	}
	logger.Debug().Str("root_id", rootID.String()).Msg("root ID computed")

	duration := time.Since(start)
	p.metrics.RootIDComputed(duration, len(executionData.ChunkExecutionDatas))

	return rootID, edRoot, errCh, nil
}

func NewExecutionDataCIDProvider(serializer execution_data.Serializer) *ExecutionDataCIDProvider {
	return &ExecutionDataCIDProvider{
		serializer:  serializer,
		maxBlobSize: execution_data.DefaultMaxBlobSize,
	}
}

type ExecutionDataCIDProvider struct {
	serializer  execution_data.Serializer
	maxBlobSize int
}

func (p *ExecutionDataCIDProvider) CalculateExecutionDataRootID(
	edRoot flow.BlockExecutionDataRoot,
) (flow.Identifier, error) {
	return p.addExecutionDataRoot(&edRoot, nil)
}

func (p *ExecutionDataCIDProvider) CalculateChunkExecutionDataID(
	ced execution_data.ChunkExecutionData,
) (cid.Cid, error) {
	return p.addChunkExecutionData(&ced, nil)
}

func (p *ExecutionDataCIDProvider) addExecutionDataRoot(
	edRoot *flow.BlockExecutionDataRoot,
	blobCh chan<- blobs.Blob,
) (flow.Identifier, error) {
	buf := new(bytes.Buffer)
	if err := p.serializer.Serialize(buf, edRoot); err != nil {
		return flow.ZeroID, fmt.Errorf("failed to serialize execution data root: %w", err)
	}

	if buf.Len() > p.maxBlobSize {
		return flow.ZeroID, errors.New("execution data root blob exceeds maximum allowed size")
	}

	rootBlob := blobs.NewBlob(buf.Bytes())
	if blobCh != nil {
		blobCh <- rootBlob
	}

	rootID, err := flow.CidToId(rootBlob.Cid())
	if err != nil {
		return flow.ZeroID, fmt.Errorf("failed to convert root blob cid to id: %w", err)
	}

	return rootID, nil
}

func (p *ExecutionDataCIDProvider) addChunkExecutionData(
	ced *execution_data.ChunkExecutionData,
	blobCh chan<- blobs.Blob,
) (cid.Cid, error) {
	cids, err := p.addBlobs(ced, blobCh)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to add chunk execution data blobs: %w", err)
	}

	for {
		if len(cids) == 1 {
			return cids[0], nil
		}

		if cids, err = p.addBlobs(cids, blobCh); err != nil {
			return cid.Undef, fmt.Errorf("failed to add cid blobs: %w", err)
		}
	}
}

// addBlobs serializes the given object, splits the serialized data into blobs, and sends them to the given channel.
func (p *ExecutionDataCIDProvider) addBlobs(v interface{}, blobCh chan<- blobs.Blob) ([]cid.Cid, error) {
	bcw := blobs.NewBlobChannelWriter(blobCh, p.maxBlobSize)
	defer bcw.Close()

	if err := p.serializer.Serialize(bcw, v); err != nil {
		return nil, fmt.Errorf("failed to serialize object: %w", err)
	}

	if err := bcw.Flush(); err != nil {
		return nil, fmt.Errorf("failed to flush blob channel writer: %w", err)
	}

	return bcw.CidsSent(), nil
}

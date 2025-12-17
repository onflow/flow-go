package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/onflow/crypto/hash"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger"
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
	if storage == nil {
		storage = &tracker.NoopStorage{}
	}

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
			cedID, err := p.cidsProvider.addChunkExecutionData(executionData.BlockID, chunkExecutionData, blobCh)
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

// GenerateExecutionDataRoot generates the execution data root and its ID from the provided
// block execution data.
// This is a helper function useful for testing.
//
// No errors are expected during normal operation.
func (p *ExecutionDataCIDProvider) GenerateExecutionDataRoot(
	executionData *execution_data.BlockExecutionData,
) (flow.Identifier, *flow.BlockExecutionDataRoot, error) {
	chunkDataIDs := make([]cid.Cid, len(executionData.ChunkExecutionDatas))
	for i, chunkExecutionData := range executionData.ChunkExecutionDatas {
		cedID, err := p.addChunkExecutionData(executionData.BlockID, chunkExecutionData, nil)
		if err != nil {
			return flow.ZeroID, nil, fmt.Errorf("failed to add chunk execution data at index %d: %w", i, err)
		}
		chunkDataIDs[i] = cedID
	}

	root := &flow.BlockExecutionDataRoot{
		BlockID:               executionData.BlockID,
		ChunkExecutionDataIDs: chunkDataIDs,
	}

	rootID, err := p.addExecutionDataRoot(root, nil)
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("failed to add execution data root: %w", err)
	}

	return rootID, root, nil
}

// CalculateExecutionDataRootID calculates the execution data root ID from the provided
// execution data root.
//
// No errors are expected during normal operation.
func (p *ExecutionDataCIDProvider) CalculateExecutionDataRootID(
	edRoot flow.BlockExecutionDataRoot,
) (flow.Identifier, error) {
	return p.addExecutionDataRoot(&edRoot, nil)
}

// CalculateChunkExecutionDataID calculates the chunk execution data ID from the provided
// chunk execution data.
//
// No errors are expected during normal operation.
func (p *ExecutionDataCIDProvider) CalculateChunkExecutionDataID(
	ced execution_data.ChunkExecutionData,
) (cid.Cid, error) {
	return p.addChunkExecutionData(flow.ZeroID, &ced, nil)
}

func (p *ExecutionDataCIDProvider) addExecutionDataRoot(
	edRoot *flow.BlockExecutionDataRoot,
	blobCh chan<- blobs.Blob,
) (flow.Identifier, error) {
	buf := new(bytes.Buffer)
	if err := p.serializer.Serialize(buf, edRoot); err != nil {
		return flow.ZeroID, fmt.Errorf("failed to serialize execution data root: %w", err)
	}

	// Debug: log the serialized root
	h := hash.NewSHA3_256()
	_, _ = h.Write(buf.Bytes())
	fmt.Printf("[DEBUG Provider] addExecutionDataRoot blockID=%x numChunks=%d serializedLen=%d serializedHash=%x\n",
		edRoot.BlockID[:], len(edRoot.ChunkExecutionDataIDs), buf.Len(), h.SumHash())
	for i, chunkCID := range edRoot.ChunkExecutionDataIDs {
		fmt.Printf("[DEBUG Provider] addExecutionDataRoot blockID=%x chunkCID[%d]=%s\n", edRoot.BlockID[:], i, chunkCID.String())
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

	fmt.Printf("[DEBUG Provider] addExecutionDataRoot blockID=%x rootID=%x rootBlobCid=%s\n", edRoot.BlockID[:], rootID[:], rootBlob.Cid().String())

	return rootID, nil
}

func (p *ExecutionDataCIDProvider) addChunkExecutionData(
	blockID flow.Identifier,
	ced *execution_data.ChunkExecutionData,
	blobCh chan<- blobs.Blob,
) (cid.Cid, error) {
	// Debug: log serialized bytes of each field
	p.logChunkExecutionDataFields(blockID, ced)

	cids, err := p.addBlobs(ced, blobCh)
	fmt.Printf("[DEBUG Provider] blockID=%x addBlobs returned %d cids\n", blockID[:], len(cids))
	for i, c := range cids {
		fmt.Printf("[DEBUG Provider] blockID=%x cid[%d]=%s\n", blockID[:], i, c.String())
	}
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

// logChunkExecutionDataFields logs the serialized bytes hash of each field in ChunkExecutionData for debugging.
func (p *ExecutionDataCIDProvider) logChunkExecutionDataFields(blockID flow.Identifier, ced *execution_data.ChunkExecutionData) {
	// Collection
	var collectionHash []byte
	if ced.Collection != nil {
		buf := new(bytes.Buffer)
		_ = p.serializer.Serialize(buf, ced.Collection)
		h := hash.NewSHA3_256()
		_, _ = h.Write(buf.Bytes())
		collectionHash = h.SumHash()
	}

	// Events
	var eventsHash []byte
	{
		buf := new(bytes.Buffer)
		_ = p.serializer.Serialize(buf, ced.Events)
		h := hash.NewSHA3_256()
		_, _ = h.Write(buf.Bytes())
		eventsHash = h.SumHash()
	}

	// TrieUpdate
	var trieUpdateHash []byte
	var trieUpdateLen int
	if ced.TrieUpdate != nil {
		buf := new(bytes.Buffer)
		_ = p.serializer.Serialize(buf, ced.TrieUpdate)
		trieUpdateLen = buf.Len()
		h := hash.NewSHA3_256()
		_, _ = h.Write(buf.Bytes())
		trieUpdateHash = h.SumHash()
	}

	// TransactionResults
	var txResultsHash []byte
	{
		buf := new(bytes.Buffer)
		_ = p.serializer.Serialize(buf, ced.TransactionResults)
		h := hash.NewSHA3_256()
		_, _ = h.Write(buf.Bytes())
		txResultsHash = h.SumHash()
	}

	// Full ChunkExecutionData
	var cedHash []byte
	var cedLen int
	var cedBytes []byte
	{
		buf := new(bytes.Buffer)
		_ = p.serializer.Serialize(buf, ced)
		cedBytes = buf.Bytes()
		cedLen = len(cedBytes)
		h := hash.NewSHA3_256()
		_, _ = h.Write(cedBytes)
		cedHash = h.SumHash()
	}

	// Log each payload CBOR individually
	if ced.TrieUpdate != nil {
		for i, payload := range ced.TrieUpdate.Payloads {
			if payload != nil {
				val := payload.Value()
				var valType string
				if val == nil {
					valType = "NIL"
				} else if len(val) == 0 {
					valType = "EMPTY_SLICE"
				} else {
					valType = fmt.Sprintf("LEN_%d", len(val))
				}
				cborBytes, _ := payload.MarshalCBOR()
				h := hash.NewSHA3_256()
				_, _ = h.Write(cborBytes)
				fmt.Printf("[DEBUG Provider] blockID=%x payload[%d] valueType=%s cborLen=%d cborHash=%x cborBytes=%x\n", blockID[:], i, valType, len(cborBytes), h.SumHash(), cborBytes)
			}
		}
	}

	// Serialize TrieUpdate using ledger.EncodeTrieUpdate for comparison
	var trieUpdateBinaryHash []byte
	var trieUpdateBinaryLen int
	if ced.TrieUpdate != nil {
		trieUpdateBytes := ledger.EncodeTrieUpdate(ced.TrieUpdate)
		trieUpdateBinaryLen = len(trieUpdateBytes)
		h := hash.NewSHA3_256()
		_, _ = h.Write(trieUpdateBytes)
		trieUpdateBinaryHash = h.SumHash()
	}

	var trieRootHash []byte
	if ced.TrieUpdate != nil {
		trieRootHash = ced.TrieUpdate.RootHash[:]
	}
	fmt.Printf("[DEBUG Provider] blockID=%x trieRootHash=%x collectionHash=%x eventsHash=%x trieUpdateHash=%x trieUpdateLen=%d txResultsHash=%x cedLen=%d cedHash=%x trieUpdateBinaryLen=%d trieUpdateBinaryHash=%x cedBytes=%x\n",
		blockID[:], trieRootHash, collectionHash, eventsHash, trieUpdateHash, trieUpdateLen, txResultsHash, cedLen, cedHash, trieUpdateBinaryLen, trieUpdateBinaryHash, cedBytes)
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

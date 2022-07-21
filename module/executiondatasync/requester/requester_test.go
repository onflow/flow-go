package requester_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	goassert "gotest.tools/assert"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/requester"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func getDatastore() datastore.Batching {
	return dssync.MutexWrap(datastore.NewMapDatastore())
}

func generateChunkExecutionData(t *testing.T, minSerializedSize uint64) *execution_data.ChunkExecutionData {
	ced := &execution_data.ChunkExecutionData{
		TrieUpdate: &ledger.TrieUpdate{
			Payloads: []*ledger.Payload{
				{
					Value: nil,
				},
			},
		},
	}

	size := 1

	for {
		buf := &bytes.Buffer{}
		require.NoError(t, execution_data.DefaultSerializer.Serialize(buf, ced))

		if buf.Len() >= int(minSerializedSize) {
			t.Logf("Chunk execution data size: %d", buf.Len())
			return ced
		}

		v := make([]byte, size)
		rand.Read(v)
		ced.TrieUpdate.Payloads[0].Value = v
		size *= 2
	}
}

func generateBlockExecutionData(t *testing.T, numChunks int, minSerializedSizePerChunk uint64) *execution_data.BlockExecutionData {
	bed := &execution_data.BlockExecutionData{
		BlockID:             unittest.IdentifierFixture(),
		ChunkExecutionDatas: make([]*execution_data.ChunkExecutionData, numChunks),
	}

	for i := 0; i < numChunks; i++ {
		bed.ChunkExecutionDatas[i] = generateChunkExecutionData(t, minSerializedSizePerChunk)
	}

	return bed
}

func TestHandleReceipt(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(0), nil).Once()

	blocks := new(mockstorage.Blocks)
	results := new(mockstorage.ExecutionResults)
	blobService := new(mocknetwork.BlobService)
	ch := make(chan struct{})
	close(ch)
	var readyCh <-chan struct{} = ch
	blobService.On("Ready").Return(readyCh)
	finalizationDistributor := pubsub.NewFinalizationDistributor()

	r, err := requester.NewRequester(
		0,
		trackerStorage,
		blocks,
		results,
		blobService,
		execution_data.DefaultSerializer,
		finalizationDistributor,
		zerolog.Nop(),
		metrics.NewNoopCollector(),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	r.Start(signalerCtx)

	block := unittest.BlockFixture()
	blocks.On("ByID", block.ID()).Return(&block, nil)
	bed := generateBlockExecutionData(t, 5, 10*execution_data.DefaultMaxBlobSize)
	blobstore := blobs.NewBlobstore(getDatastore())
	eds := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)
	rootID, err := eds.AddExecutionData(context.Background(), bed)
	require.NoError(t, err)
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutionResult.ExecutionDataID = rootID
	receipt.ExecutionResult.BlockID = block.ID()

	blobGetter := new(mocknetwork.BlobGetter)
	blobService.On("GetSession", mock.Anything).Return(blobGetter, nil)

	blobGetter.On("GetBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(ctx context.Context, c cid.Cid) blobs.Blob {
			blob, _ := blobstore.Get(ctx, c)
			return blob
		},
		func(ctx context.Context, c cid.Cid) error {
			_, err := blobstore.Get(ctx, c)
			return err
		},
	)
	blobGetter.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).Return(
		func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			blobCh := make(chan blobs.Blob, len(cids))
			for _, c := range cids {
				blob, err := blobstore.Get(ctx, c)
				assert.NoError(t, err)
				blobCh <- blob
			}
			close(blobCh)
			return blobCh
		},
	)

	trackerStorage.On("Update", mock.AnythingOfType("tracker.UpdateFn")).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error {
			return nil
		})
	})

	<-r.Ready()

	r.HandleReceipt(receipt)

	// allow some time for the request to propagate
	time.Sleep(time.Second)

	blocks.AssertExpectations(t)
	blobGetter.AssertExpectations(t)
	blobService.AssertExpectations(t)

	cancel()
	<-r.Done()

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}

func TestHandleFinalizedBlock(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(0), nil).Once()

	blocks := new(mockstorage.Blocks)
	results := new(mockstorage.ExecutionResults)
	blobService := new(mocknetwork.BlobService)
	ch := make(chan struct{})
	close(ch)
	var readyCh <-chan struct{} = ch
	blobService.On("Ready").Return(readyCh)
	finalizationDistributor := pubsub.NewFinalizationDistributor()

	r, err := requester.NewRequester(
		0,
		trackerStorage,
		blocks,
		results,
		blobService,
		execution_data.DefaultSerializer,
		finalizationDistributor,
		zerolog.Nop(),
		metrics.NewNoopCollector(),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	r.Start(signalerCtx)

	sealedBlock := unittest.BlockFixture()
	bed := generateBlockExecutionData(t, 5, 10*execution_data.DefaultMaxBlobSize)
	bed.BlockID = sealedBlock.ID()
	blobstore := blobs.NewBlobstore(getDatastore())
	eds := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)
	rootID, err := eds.AddExecutionData(context.Background(), bed)
	require.NoError(t, err)
	result := unittest.ExecutionResultFixture()
	result.ExecutionDataID = rootID
	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(result), unittest.Seal.WithBlock(sealedBlock.Header))
	finalizedPayload := unittest.PayloadFixture(unittest.WithSeals(seal))
	finalizedHeader := unittest.BlockHeaderFixture()
	finalizedHeader.PayloadHash = finalizedPayload.Hash()
	finalizedBlock := &flow.Block{
		Header:  finalizedHeader,
		Payload: &finalizedPayload,
	}
	blocks.On("ByID", finalizedBlock.ID()).Return(finalizedBlock, nil)
	blocks.On("ByID", sealedBlock.ID()).Return(&sealedBlock, nil)
	results.On("ByID", result.ID()).Return(result, nil)

	blobGetter := new(mocknetwork.BlobGetter)
	blobService.On("GetSession", mock.Anything).Return(blobGetter, nil)

	blobGetter.On("GetBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(ctx context.Context, c cid.Cid) blobs.Blob {
			blob, _ := blobstore.Get(ctx, c)
			return blob
		},
		func(ctx context.Context, c cid.Cid) error {
			_, err := blobstore.Get(ctx, c)
			return err
		},
	)
	blobGetter.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).Return(
		func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			blobCh := make(chan blobs.Blob, len(cids))
			for _, c := range cids {
				blob, err := blobstore.Get(ctx, c)
				assert.NoError(t, err)
				blobCh <- blob
			}
			close(blobCh)
			return blobCh
		},
	)

	trackerStorage.On("Update", mock.AnythingOfType("tracker.UpdateFn")).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error {
			return nil
		})
	})

	<-r.Ready()

	finalizationDistributor.OnFinalizedBlock(&model.Block{
		View:        finalizedBlock.Header.View,
		BlockID:     finalizedBlock.ID(),
		ProposerID:  finalizedBlock.Header.ProposerID,
		Timestamp:   finalizedBlock.Header.Timestamp,
		PayloadHash: finalizedBlock.Payload.Hash(),
	})

	// allow some time for the request to propagate
	time.Sleep(time.Second)

	blocks.AssertExpectations(t)
	results.AssertExpectations(t)
	blobGetter.AssertExpectations(t)
	blobService.AssertExpectations(t)

	cancel()
	<-r.Done()

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}

func TestConsumer(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(0), nil).Once()

	blocks := new(mockstorage.Blocks)
	results := new(mockstorage.ExecutionResults)
	blobService := new(mocknetwork.BlobService)
	ch := make(chan struct{})
	close(ch)
	var readyCh <-chan struct{} = ch
	blobService.On("Ready").Return(readyCh)
	finalizationDistributor := pubsub.NewFinalizationDistributor()

	r, err := requester.NewRequester(
		0,
		trackerStorage,
		blocks,
		results,
		blobService,
		execution_data.DefaultSerializer,
		finalizationDistributor,
		zerolog.Nop(),
		metrics.NewNoopCollector(),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	blobstore := blobs.NewBlobstore(getDatastore())
	eds := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	r.Start(signalerCtx)

	sealedBlock := unittest.BlockFixture()
	sealedBlock.Header.Height = 1
	bed := generateBlockExecutionData(t, 5, 10*execution_data.DefaultMaxBlobSize)
	bed.BlockID = sealedBlock.ID()
	rootID, err := eds.AddExecutionData(context.Background(), bed)
	require.NoError(t, err)
	result := unittest.ExecutionResultFixture()
	result.ExecutionDataID = rootID
	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(result), unittest.Seal.WithBlock(sealedBlock.Header))
	finalizedPayload := unittest.PayloadFixture(unittest.WithSeals(seal))
	finalizedHeader := unittest.BlockHeaderFixture()
	finalizedHeader.PayloadHash = finalizedPayload.Hash()
	finalizedBlock := &flow.Block{
		Header:  finalizedHeader,
		Payload: &finalizedPayload,
	}
	blocks.On("ByID", finalizedBlock.ID()).Return(finalizedBlock, nil)
	blocks.On("ByID", sealedBlock.ID()).Return(&sealedBlock, nil)
	results.On("ByID", result.ID()).Return(result, nil)

	blobGetter := new(mocknetwork.BlobGetter)
	blobService.On("GetSession", mock.Anything).Return(blobGetter, nil)

	blobGetter.On("GetBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(ctx context.Context, c cid.Cid) blobs.Blob {
			blob, _ := blobstore.Get(ctx, c)
			return blob
		},
		func(ctx context.Context, c cid.Cid) error {
			_, err := blobstore.Get(ctx, c)
			return err
		},
	)
	blobGetter.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).Return(
		func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			blobCh := make(chan blobs.Blob, len(cids))
			for _, c := range cids {
				blob, err := blobstore.Get(ctx, c)
				assert.NoError(t, err)
				blobCh <- blob
			}
			close(blobCh)
			return blobCh
		},
	)

	trackerStorage.On("Update", mock.AnythingOfType("tracker.UpdateFn")).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error {
			return nil
		})
	})
	trackerStorage.On("SetFulfilledHeight", uint64(1)).Return(nil)

	<-r.Ready()

	numCalls := atomic.NewInt32(0)
	var executionData *execution_data.BlockExecutionData
	unsub, err := r.AddConsumer(func(blockHeight uint64, ed *execution_data.BlockExecutionData) {
		assert.Equal(t, blockHeight, sealedBlock.Header.Height)
		executionData = ed
		numCalls.Add(1)
	})
	require.NoError(t, err)

	finalizationDistributor.OnFinalizedBlock(&model.Block{
		View:        finalizedBlock.Header.View,
		BlockID:     finalizedBlock.ID(),
		ProposerID:  finalizedBlock.Header.ProposerID,
		Timestamp:   finalizedBlock.Header.Timestamp,
		PayloadHash: finalizedBlock.Payload.Hash(),
	})
	assert.Eventually(t, func() bool {
		return numCalls.Load() == 1
	}, time.Second, 100*time.Millisecond)

	blocks.AssertExpectations(t)
	results.AssertExpectations(t)
	blobGetter.AssertExpectations(t)
	blobService.AssertExpectations(t)
	trackerStorage.AssertExpectations(t)
	goassert.DeepEqual(t, bed, executionData)

	sealedBlock = unittest.BlockFixture()
	sealedBlock.Header.Height = 2
	bed = generateBlockExecutionData(t, 5, 10*execution_data.DefaultMaxBlobSize)
	bed.BlockID = sealedBlock.ID()
	rootID, err = eds.AddExecutionData(context.Background(), bed)
	require.NoError(t, err)
	result = unittest.ExecutionResultFixture()
	result.ExecutionDataID = rootID
	seal = unittest.Seal.Fixture(unittest.Seal.WithResult(result), unittest.Seal.WithBlock(sealedBlock.Header))
	finalizedPayload = unittest.PayloadFixture(unittest.WithSeals(seal))
	finalizedHeader = unittest.BlockHeaderFixture()
	finalizedHeader.PayloadHash = finalizedPayload.Hash()
	finalizedBlock = &flow.Block{
		Header:  finalizedHeader,
		Payload: &finalizedPayload,
	}
	blocks.On("ByID", finalizedBlock.ID()).Return(finalizedBlock, nil)
	blocks.On("ByID", sealedBlock.ID()).Return(&sealedBlock, nil)
	results.On("ByID", result.ID()).Return(result, nil)
	trackerStorage.On("SetFulfilledHeight", uint64(2)).Return(nil)

	err = unsub()
	require.NoError(t, err)

	finalizationDistributor.OnFinalizedBlock(&model.Block{
		View:        finalizedBlock.Header.View,
		BlockID:     finalizedBlock.ID(),
		ProposerID:  finalizedBlock.Header.ProposerID,
		Timestamp:   finalizedBlock.Header.Timestamp,
		PayloadHash: finalizedBlock.Payload.Hash(),
	})
	assert.Never(t, func() bool {
		return numCalls.Load() != 1
	}, time.Second, 100*time.Millisecond)

	blocks.AssertExpectations(t)
	results.AssertExpectations(t)
	blobGetter.AssertExpectations(t)
	blobService.AssertExpectations(t)
	trackerStorage.AssertExpectations(t)

	cancel()
	select {
	case <-r.Done():
	case err := <-errChan:
		require.NoError(t, err)
	}

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}

func TestStaleReceipt(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(0), nil).Once()

	blocks := new(mockstorage.Blocks)
	results := new(mockstorage.ExecutionResults)
	blobService := new(mocknetwork.BlobService)
	ch := make(chan struct{})
	close(ch)
	var readyCh <-chan struct{} = ch
	blobService.On("Ready").Return(readyCh)
	finalizationDistributor := pubsub.NewFinalizationDistributor()

	r, err := requester.NewRequester(
		0,
		trackerStorage,
		blocks,
		results,
		blobService,
		execution_data.DefaultSerializer,
		finalizationDistributor,
		zerolog.Nop(),
		metrics.NewNoopCollector(),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	r.Start(signalerCtx)

	sealedBlock := unittest.BlockFixture()
	sealedBlock.Header.Height = 1
	bed := generateBlockExecutionData(t, 5, 10*execution_data.DefaultMaxBlobSize)
	bed.BlockID = sealedBlock.ID()
	blobstore := blobs.NewBlobstore(getDatastore())
	eds := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)
	rootID, err := eds.AddExecutionData(context.Background(), bed)
	require.NoError(t, err)
	result := unittest.ExecutionResultFixture()
	result.ExecutionDataID = rootID
	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(result), unittest.Seal.WithBlock(sealedBlock.Header))
	finalizedPayload := unittest.PayloadFixture(unittest.WithSeals(seal))
	finalizedHeader := unittest.BlockHeaderFixture()
	finalizedHeader.PayloadHash = finalizedPayload.Hash()
	finalizedBlock := &flow.Block{
		Header:  finalizedHeader,
		Payload: &finalizedPayload,
	}
	blocks.On("ByID", finalizedBlock.ID()).Return(finalizedBlock, nil)
	blocks.On("ByID", sealedBlock.ID()).Return(&sealedBlock, nil)
	results.On("ByID", result.ID()).Return(result, nil)

	blobGetter := new(mocknetwork.BlobGetter)
	blobService.On("GetSession", mock.Anything).Return(blobGetter, nil).Once()

	blobGetter.On("GetBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(ctx context.Context, c cid.Cid) blobs.Blob {
			blob, _ := blobstore.Get(ctx, c)
			return blob
		},
		func(ctx context.Context, c cid.Cid) error {
			_, err := blobstore.Get(ctx, c)
			return err
		},
	)
	blobGetter.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).Return(
		func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			blobCh := make(chan blobs.Blob, len(cids))
			for _, c := range cids {
				blob, err := blobstore.Get(ctx, c)
				assert.NoError(t, err)
				blobCh <- blob
			}
			close(blobCh)
			return blobCh
		},
	)

	trackerStorage.On("Update", mock.AnythingOfType("tracker.UpdateFn")).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error {
			return nil
		})
	})
	trackerStorage.On("SetFulfilledHeight", uint64(1)).Return(nil)

	<-r.Ready()

	finalizationDistributor.OnFinalizedBlock(&model.Block{
		View:        finalizedBlock.Header.View,
		BlockID:     finalizedBlock.ID(),
		ProposerID:  finalizedBlock.Header.ProposerID,
		Timestamp:   finalizedBlock.Header.Timestamp,
		PayloadHash: finalizedBlock.Payload.Hash(),
	})

	// allow some time for the request to propagate
	time.Sleep(time.Second)

	blocks.AssertExpectations(t)
	results.AssertExpectations(t)
	blobGetter.AssertExpectations(t)
	blobService.AssertExpectations(t)

	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutionResult.ExecutionDataID = unittest.IdentifierFixture()
	receipt.ExecutionResult.BlockID = sealedBlock.ID()
	r.HandleReceipt(receipt)

	// allow some time for the request to propagate
	// if the blob service is called during this time,
	// the mock would throw an error
	time.Sleep(time.Second)

	blobService.AssertExpectations(t)

	cancel()
	<-r.Done()

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}

func TestOversizedBlob(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(0), nil).Once()

	blocks := new(mockstorage.Blocks)
	results := new(mockstorage.ExecutionResults)
	blobService := new(mocknetwork.BlobService)
	ch := make(chan struct{})
	close(ch)
	var readyCh <-chan struct{} = ch
	blobService.On("Ready").Return(readyCh)
	finalizationDistributor := pubsub.NewFinalizationDistributor()

	r, err := requester.NewRequester(
		0,
		trackerStorage,
		blocks,
		results,
		blobService,
		execution_data.DefaultSerializer,
		finalizationDistributor,
		zerolog.Nop(),
		metrics.NewNoopCollector(),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	r.Start(signalerCtx)

	sealedBlock := unittest.BlockFixture()
	sealedBlock.Header.Height = 1
	bed := generateBlockExecutionData(t, 5, 10*execution_data.DefaultMaxBlobSize)
	bed.BlockID = sealedBlock.ID()
	blobstore := blobs.NewBlobstore(getDatastore())
	eds := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer, execution_data.WithMaxBlobSize(2*execution_data.DefaultMaxBlobSize))
	rootID, err := eds.AddExecutionData(context.Background(), bed)
	require.NoError(t, err)
	result := unittest.ExecutionResultFixture()
	result.ExecutionDataID = rootID
	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(result), unittest.Seal.WithBlock(sealedBlock.Header))
	finalizedPayload := unittest.PayloadFixture(unittest.WithSeals(seal))
	finalizedHeader := unittest.BlockHeaderFixture()
	finalizedHeader.PayloadHash = finalizedPayload.Hash()
	finalizedBlock := &flow.Block{
		Header:  finalizedHeader,
		Payload: &finalizedPayload,
	}
	blocks.On("ByID", finalizedBlock.ID()).Return(finalizedBlock, nil)
	blocks.On("ByID", sealedBlock.ID()).Return(&sealedBlock, nil)
	results.On("ByID", result.ID()).Return(result, nil)

	blobGetter := new(mocknetwork.BlobGetter)
	blobService.On("GetSession", mock.Anything).Return(blobGetter, nil).Once()

	blobGetter.On("GetBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(ctx context.Context, c cid.Cid) blobs.Blob {
			blob, _ := blobstore.Get(ctx, c)
			return blob
		},
		func(ctx context.Context, c cid.Cid) error {
			_, err := blobstore.Get(ctx, c)
			return err
		},
	)
	blobGetter.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).Return(
		func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			blobCh := make(chan blobs.Blob, len(cids))
			for _, c := range cids {
				blob, err := blobstore.Get(ctx, c)
				assert.NoError(t, err)
				blobCh <- blob
			}
			close(blobCh)
			return blobCh
		},
	)

	trackerStorage.On("Update", mock.AnythingOfType("tracker.UpdateFn")).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error {
			return nil
		})
	})

	<-r.Ready()

	finalizationDistributor.OnFinalizedBlock(&model.Block{
		View:        finalizedBlock.Header.View,
		BlockID:     finalizedBlock.ID(),
		ProposerID:  finalizedBlock.Header.ProposerID,
		Timestamp:   finalizedBlock.Header.Timestamp,
		PayloadHash: finalizedBlock.Payload.Hash(),
	})

	err = <-errChan
	var blobSizeLimitExceededErr *requester.BlobSizeLimitExceededError
	assert.ErrorAs(t, err, &blobSizeLimitExceededErr)

	blocks.AssertExpectations(t)
	results.AssertExpectations(t)
	blobGetter.AssertExpectations(t)
	blobService.AssertExpectations(t)

	cancel()
	<-r.Done()
}

type randomSerializer struct{}

func (rs *randomSerializer) Serialize(w io.Writer, v interface{}) error {
	data := make([]byte, 1024)
	rand.Read(data)
	_, err := w.Write(data)
	return err
}

func (rs *randomSerializer) Deserialize(r io.Reader) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

type corruptedTailSerializer struct {
	corruptedChunk int
	i              int
}

func newCorruptedTailSerializer(numChunks int) *corruptedTailSerializer {
	return &corruptedTailSerializer{
		corruptedChunk: rand.Intn(numChunks) + 1,
	}
}

func (cts *corruptedTailSerializer) Serialize(w io.Writer, v interface{}) error {
	if _, ok := v.(*execution_data.ChunkExecutionData); ok {
		cts.i++
		if cts.i == cts.corruptedChunk {
			buf := &bytes.Buffer{}

			err := execution_data.DefaultSerializer.Serialize(buf, v)
			if err != nil {
				return err
			}

			data := buf.Bytes()
			rand.Read(data[len(data)-1024:])

			_, err = w.Write(data)
			return err
		}
	}

	return execution_data.DefaultSerializer.Serialize(w, v)
}

func (cts *corruptedTailSerializer) Deserialize(r io.Reader) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func runWithSerializer(t *testing.T, serializer execution_data.Serializer) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(0), nil).Once()

	blocks := new(mockstorage.Blocks)
	results := new(mockstorage.ExecutionResults)
	blobService := new(mocknetwork.BlobService)
	ch := make(chan struct{})
	close(ch)
	var readyCh <-chan struct{} = ch
	blobService.On("Ready").Return(readyCh)
	finalizationDistributor := pubsub.NewFinalizationDistributor()

	r, err := requester.NewRequester(
		0,
		trackerStorage,
		blocks,
		results,
		blobService,
		execution_data.DefaultSerializer,
		finalizationDistributor,
		zerolog.Nop(),
		metrics.NewNoopCollector(),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	r.Start(signalerCtx)

	sealedBlock := unittest.BlockFixture()
	sealedBlock.Header.Height = 1
	bed := generateBlockExecutionData(t, 5, 10*execution_data.DefaultMaxBlobSize)
	bed.BlockID = sealedBlock.ID()
	blobstore := blobs.NewBlobstore(getDatastore())
	eds := execution_data.NewExecutionDataStore(blobstore, serializer)
	rootID, err := eds.AddExecutionData(context.Background(), bed)
	require.NoError(t, err)
	result := unittest.ExecutionResultFixture()
	result.ExecutionDataID = rootID
	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(result), unittest.Seal.WithBlock(sealedBlock.Header))
	finalizedPayload := unittest.PayloadFixture(unittest.WithSeals(seal))
	finalizedHeader := unittest.BlockHeaderFixture()
	finalizedHeader.PayloadHash = finalizedPayload.Hash()
	finalizedBlock := &flow.Block{
		Header:  finalizedHeader,
		Payload: &finalizedPayload,
	}
	blocks.On("ByID", finalizedBlock.ID()).Return(finalizedBlock, nil)
	blocks.On("ByID", sealedBlock.ID()).Return(&sealedBlock, nil)
	results.On("ByID", result.ID()).Return(result, nil)

	blobGetter := new(mocknetwork.BlobGetter)
	blobService.On("GetSession", mock.Anything).Return(blobGetter, nil).Once()

	blobGetter.On("GetBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(ctx context.Context, c cid.Cid) blobs.Blob {
			blob, _ := blobstore.Get(ctx, c)
			return blob
		},
		func(ctx context.Context, c cid.Cid) error {
			_, err := blobstore.Get(ctx, c)
			return err
		},
	)
	blobGetter.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).Return(
		func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			blobCh := make(chan blobs.Blob, len(cids))
			for _, c := range cids {
				blob, err := blobstore.Get(ctx, c)
				assert.NoError(t, err)
				blobCh <- blob
			}
			close(blobCh)
			return blobCh
		},
	).Maybe()

	trackerStorage.On("Update", mock.AnythingOfType("tracker.UpdateFn")).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error {
			return nil
		})
	})

	<-r.Ready()

	finalizationDistributor.OnFinalizedBlock(&model.Block{
		View:        finalizedBlock.Header.View,
		BlockID:     finalizedBlock.ID(),
		ProposerID:  finalizedBlock.Header.ProposerID,
		Timestamp:   finalizedBlock.Header.Timestamp,
		PayloadHash: finalizedBlock.Payload.Hash(),
	})

	err = <-errChan
	var malformedDataErr *execution_data.MalformedDataError
	assert.ErrorAs(t, err, &malformedDataErr)

	blocks.AssertExpectations(t)
	results.AssertExpectations(t)
	blobGetter.AssertExpectations(t)
	blobService.AssertExpectations(t)

	cancel()
	<-r.Done()
}

func TestMalformedData(t *testing.T) {
	runWithSerializer(t, &randomSerializer{})
	runWithSerializer(t, newCorruptedTailSerializer(5))
}

func TestMismatchedBlockID(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(0), nil).Once()

	blocks := new(mockstorage.Blocks)
	results := new(mockstorage.ExecutionResults)
	blobService := new(mocknetwork.BlobService)
	ch := make(chan struct{})
	close(ch)
	var readyCh <-chan struct{} = ch
	blobService.On("Ready").Return(readyCh)
	finalizationDistributor := pubsub.NewFinalizationDistributor()

	r, err := requester.NewRequester(
		0,
		trackerStorage,
		blocks,
		results,
		blobService,
		execution_data.DefaultSerializer,
		finalizationDistributor,
		zerolog.Nop(),
		metrics.NewNoopCollector(),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	r.Start(signalerCtx)

	sealedBlock := unittest.BlockFixture()
	sealedBlock.Header.Height = 1
	bed := generateBlockExecutionData(t, 5, 10*execution_data.DefaultMaxBlobSize)
	blobstore := blobs.NewBlobstore(getDatastore())
	eds := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)
	rootID, err := eds.AddExecutionData(context.Background(), bed)
	require.NoError(t, err)
	result := unittest.ExecutionResultFixture()
	result.ExecutionDataID = rootID
	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(result), unittest.Seal.WithBlock(sealedBlock.Header))
	finalizedPayload := unittest.PayloadFixture(unittest.WithSeals(seal))
	finalizedHeader := unittest.BlockHeaderFixture()
	finalizedHeader.PayloadHash = finalizedPayload.Hash()
	finalizedBlock := &flow.Block{
		Header:  finalizedHeader,
		Payload: &finalizedPayload,
	}
	blocks.On("ByID", finalizedBlock.ID()).Return(finalizedBlock, nil)
	blocks.On("ByID", sealedBlock.ID()).Return(&sealedBlock, nil)
	results.On("ByID", result.ID()).Return(result, nil)

	blobGetter := new(mocknetwork.BlobGetter)
	blobService.On("GetSession", mock.Anything).Return(blobGetter, nil).Once()

	blobGetter.On("GetBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(ctx context.Context, c cid.Cid) blobs.Blob {
			blob, _ := blobstore.Get(ctx, c)
			return blob
		},
		func(ctx context.Context, c cid.Cid) error {
			_, err := blobstore.Get(ctx, c)
			return err
		},
	)
	blobGetter.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).Return(
		func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			blobCh := make(chan blobs.Blob, len(cids))
			for _, c := range cids {
				blob, err := blobstore.Get(ctx, c)
				assert.NoError(t, err)
				blobCh <- blob
			}
			close(blobCh)
			return blobCh
		},
	)

	trackerStorage.On("Update", mock.AnythingOfType("tracker.UpdateFn")).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error {
			return nil
		})
	})

	<-r.Ready()

	finalizationDistributor.OnFinalizedBlock(&model.Block{
		View:        finalizedBlock.Header.View,
		BlockID:     finalizedBlock.ID(),
		ProposerID:  finalizedBlock.Header.ProposerID,
		Timestamp:   finalizedBlock.Header.Timestamp,
		PayloadHash: finalizedBlock.Payload.Hash(),
	})

	err = <-errChan
	var mismatchedBlockIDErr *requester.MismatchedBlockIDError
	assert.ErrorAs(t, err, &mismatchedBlockIDErr)

	blocks.AssertExpectations(t)
	results.AssertExpectations(t)
	blobGetter.AssertExpectations(t)
	blobService.AssertExpectations(t)

	cancel()
	<-r.Done()
}

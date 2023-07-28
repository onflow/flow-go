package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	goassert "gotest.tools/assert"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

func getDatastore() datastore.Batching {
	return dssync.MutexWrap(datastore.NewMapDatastore())
}

func getExecutionDataStore(ds datastore.Batching) execution_data.ExecutionDataStore {
	return execution_data.NewExecutionDataStore(blobs.NewBlobstore(ds), execution_data.DefaultSerializer)
}

func getBlobservice(ds datastore.Batching) network.BlobService {
	blobstore := blobs.NewBlobstore(ds)
	blobService := new(mocknetwork.BlobService)
	blobService.On("AddBlobs", mock.Anything, mock.AnythingOfType("[]blocks.Block")).Return(blobstore.PutMany)
	return blobService
}

func getProvider(blobService network.BlobService) provider.Provider {
	trackerStorage := mocktracker.NewMockStorage()

	return provider.NewProvider(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		blobService,
		trackerStorage,
	)
}

func generateBlockExecutionData(t *testing.T, numChunks int, minSerializedSizePerChunk uint64) *execution_data.BlockExecutionData {
	chunkData := make([]*execution_data.ChunkExecutionData, 0, numChunks)
	for i := 0; i < numChunks; i++ {
		chunkData = append(chunkData, unittest.ChunkExecutionDataFixture(t, int(minSerializedSizePerChunk)))
	}

	return unittest.BlockExecutionDataFixture(unittest.WithChunkExecutionDatas(chunkData...))
}

func deepEqual(t *testing.T, expected, actual *execution_data.BlockExecutionData) {
	assert.Equal(t, expected.BlockID, actual.BlockID)
	assert.Equal(t, len(expected.ChunkExecutionDatas), len(actual.ChunkExecutionDatas))

	for i, expectedChunk := range expected.ChunkExecutionDatas {
		actualChunk := actual.ChunkExecutionDatas[i]

		goassert.DeepEqual(t, expectedChunk.Collection, actualChunk.Collection)
		goassert.DeepEqual(t, expectedChunk.Events, actualChunk.Events)
		assert.True(t, expectedChunk.TrieUpdate.Equals(actualChunk.TrieUpdate))
	}
}

func TestHappyPath(t *testing.T) {
	t.Parallel()

	ds := getDatastore()
	provider := getProvider(getBlobservice(ds))
	store := getExecutionDataStore(ds)

	test := func(numChunks int, minSerializedSizePerChunk uint64) {
		expected := generateBlockExecutionData(t, numChunks, minSerializedSizePerChunk)
		executionDataID, executionDataRoot, err := provider.Provide(context.Background(), 0, expected)
		require.NoError(t, err)
		actual, err := store.Get(context.Background(), executionDataID)
		require.NoError(t, err)
		deepEqual(t, expected, actual)
		assert.Equal(t, expected.BlockID, executionDataRoot.BlockID)
	}

	test(1, 0)                                   // small execution data (single level blob tree)
	test(5, 5*execution_data.DefaultMaxBlobSize) // large execution data (multi level blob tree)
}

func TestProvideContextCanceled(t *testing.T) {
	t.Parallel()

	bed := generateBlockExecutionData(t, 5, 5*execution_data.DefaultMaxBlobSize)

	provider := getProvider(getBlobservice(getDatastore()))
	_, _, err := provider.Provide(context.Background(), 0, bed)
	require.NoError(t, err)

	blobService := new(mocknetwork.BlobService)
	blobService.On("AddBlobs", mock.Anything, mock.AnythingOfType("[]blocks.Block")).
		Return(func(ctx context.Context, blobs []blobs.Blob) error {
			<-ctx.Done()
			return ctx.Err()
		})
	provider = getProvider(blobService)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err = provider.Provide(ctx, 0, bed)
	assert.ErrorIs(t, err, ctx.Err())
}

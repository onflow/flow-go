package provider_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	goassert "gotest.tools/assert"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
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
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("Update", mock.Anything).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

	return provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		blobService,
		trackerStorage,
	)
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

func TestHappyPath(t *testing.T) {
	t.Parallel()

	ds := getDatastore()
	provider := getProvider(getBlobservice(ds))
	store := getExecutionDataStore(ds)

	test := func(numChunks int, minSerializedSizePerChunk uint64) {
		expected := generateBlockExecutionData(t, numChunks, minSerializedSizePerChunk)
		executionDataID, _, err := provider.Provide(context.Background(), 0, expected)
		require.NoError(t, err)
		actual, err := store.GetExecutionData(context.Background(), executionDataID)
		require.NoError(t, err)
		goassert.DeepEqual(t, expected, actual)
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

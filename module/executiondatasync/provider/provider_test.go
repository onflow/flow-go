package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	goassert "gotest.tools/assert"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
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

func getBlobservice(t *testing.T, ds datastore.Batching) network.BlobService {
	blobstore := blobs.NewBlobstore(ds)
	blobService := mocknetwork.NewBlobService(t)
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
	provider := getProvider(getBlobservice(t, ds))
	store := getExecutionDataStore(ds)

	test := func(numChunks int, minSerializedSizePerChunk uint64) {
		expected := generateBlockExecutionData(t, numChunks, minSerializedSizePerChunk)
		executionDataID, executionDataRoot, err := provider.Provide(context.Background(), 0, expected)
		require.NoError(t, err)

		actual, err := store.Get(context.Background(), executionDataID)
		require.NoError(t, err)
		deepEqual(t, expected, actual)

		assert.Equal(t, expected.BlockID, executionDataRoot.BlockID)
		assert.Len(t, executionDataRoot.ChunkExecutionDataIDs, numChunks)
	}

	test(1, 0)                                   // small execution data (single level blob tree)
	test(5, 5*execution_data.DefaultMaxBlobSize) // large execution data (multi level blob tree)
}

func TestProvideContextCanceled(t *testing.T) {
	t.Parallel()

	bed := generateBlockExecutionData(t, 5, 5*execution_data.DefaultMaxBlobSize)

	provider := getProvider(getBlobservice(t, getDatastore()))
	_, _, err := provider.Provide(context.Background(), 0, bed)
	require.NoError(t, err)

	blobService := mocknetwork.NewBlobService(t)
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

// TestCalculateExecutionDataRootID tests that CalculateExecutionDataRootID calculates the correct ID given a static BlockExecutionDataRoot
func TestCalculateExecutionDataRootID(t *testing.T) {
	t.Parallel()

	expected := flow.MustHexStringToIdentifier("ae80bb200545de7ff009d2f3e20970643198be635a9b90fffb9da1198a988deb")
	edRoot := flow.BlockExecutionDataRoot{
		BlockID: flow.MustHexStringToIdentifier("2b31c5e26b999a41d18dc62584ac68476742b071fc9412d68be9e516e1dfc79e"),
		ChunkExecutionDataIDs: []cid.Cid{
			cid.MustParse("QmcA2h3jARWXkCc9VvpR4jvt9cNc7RdiqSMvPJ1TU69Xvw"),
			cid.MustParse("QmQN81Y7KdHWNdsLthDxtdf2dCHLb3ddjDWmDZQ4Znqfs4"),
			cid.MustParse("QmcfMmNPa8jFN64t1Hu7Afk7Trx8a6dg7gZfEEUqzC827b"),
			cid.MustParse("QmYTooZGux6epKrdHbzgubUN4JFHkLK9hw6Z6F3fAMEDH5"),
			cid.MustParse("QmXxYakkZKZEoCVdLLzVisctMxyiWQSfassMMzvCdaCjAj"),
		},
	}

	cidProvider := provider.NewExecutionDataCIDProvider(execution_data.DefaultSerializer)
	actual, err := cidProvider.CalculateExecutionDataRootID(edRoot)
	require.NoError(t, err)

	assert.Equal(t, expected, actual)
}

// TestCalculateChunkExecutionDataID tests that CalculateChunkExecutionDataID calculates the correct ID given a static ChunkExecutionData
// This is used to ensure library updates or modification to the provider do not change the ID calculation logic
func TestCalculateChunkExecutionDataID(t *testing.T) {
	t.Parallel()

	rootHash, err := ledger.ToRootHash([]byte("0123456789acbdef0123456789acbdef"))
	require.NoError(t, err)

	expected := cid.MustParse("QmdtRuw9jFgkynBWofz4qFQHDqUwLhi2nReF4fUyXvdERC")
	ced := execution_data.ChunkExecutionData{
		Collection: &flow.Collection{
			Transactions: []*flow.TransactionBody{
				{Script: []byte("access(all) fun main() {}")},
			},
		},
		Events: []flow.Event{
			unittest.EventFixture(flow.EventType("A.0123456789abcdef.SomeContract.SomeEvent"), 1, 2, flow.MustHexStringToIdentifier("95e0929839063afbe334a3d175bea0775cdf5d93f64299e369d16ce21aa423d3"), 0),
		},
		TrieUpdate: &ledger.TrieUpdate{
			RootHash: rootHash,
		},
		TransactionResults: []flow.LightTransactionResult{
			{
				TransactionID:   flow.MustHexStringToIdentifier("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
				ComputationUsed: 100,
				Failed:          true,
			},
		},
	}

	cidProvider := provider.NewExecutionDataCIDProvider(execution_data.DefaultSerializer)
	actual, err := cidProvider.CalculateChunkExecutionDataID(ced)
	require.NoError(t, err)

	// This can be used for updating the expected ID when the format is *intentionally* updated
	t.Log(actual)

	assert.Equal(t, expected, actual)
}

package provider_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	goassert "gotest.tools/assert"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	edstorage "github.com/onflow/flow-go/module/executiondatasync/storage"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

const (
	// canonicalGeneratorSeed is the static randomness seed used to generate the canonical execution data.
	// this ensures the data is deterministic and can be used for hash testing.
	canonicalGeneratorSeed = 1103801176737782919
)

var (
	// canonicalExecutionDataID is the execution data ID of the canonical execution data.
	canonicalExecutionDataID = flow.MustHexStringToIdentifier("15fb366a043645f201587047a52a965f42013e202fb686cbe1d595554b1d2126")
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

func generateBlockExecutionData(numChunks int, minSerializedSizePerChunk int) *execution_data.BlockExecutionData {
	suite := fixtures.NewGeneratorSuite()

	cedGen := suite.ChunkExecutionDatas()
	chunkExecutionData := cedGen.List(numChunks, cedGen.WithMinSize(minSerializedSizePerChunk))

	bedGen := suite.BlockExecutionDatas()
	return bedGen.Fixture(bedGen.WithChunkExecutionDatas(chunkExecutionData...))
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

	test := func(numChunks int, minSerializedSizePerChunk int) {
		expected := generateBlockExecutionData(numChunks, minSerializedSizePerChunk)
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

	bed := generateBlockExecutionData(5, 5*execution_data.DefaultMaxBlobSize)

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

// TestGenerateExecutionDataRoot tests that GenerateExecutionDataRoot produces the same execution data ID and root
// as the Provide method.
// This ensures we can use the GenerateExecutionDataRoot method during testing to generate the correct data.
func TestGenerateExecutionDataRoot(t *testing.T) {
	t.Parallel()

	bed := generateBlockExecutionData(5, 5*execution_data.DefaultMaxBlobSize)

	testProvider := getProvider(getBlobservice(t, getDatastore()))
	expectedExecutionDataID, expectedExecutionDataRoot, err := testProvider.Provide(context.Background(), 0, bed)
	require.NoError(t, err)

	cidProvider := provider.NewExecutionDataCIDProvider(execution_data.DefaultSerializer)
	actualExecutionDataID, actualExecutionDataRoot, err := cidProvider.GenerateExecutionDataRoot(bed)
	require.NoError(t, err)

	assert.Equal(t, expectedExecutionDataID, actualExecutionDataID)
	assert.Equal(t, expectedExecutionDataRoot, actualExecutionDataRoot)
}

// TestCalculateExecutionDataRootID tests that CalculateExecutionDataRootID calculates the correct
// ID given a static BlockExecutionDataRoot. This is used to ensure library updates or modification
// to the provider do not change the ID calculation logic.
//
// CAUTION: Unintentional changes may cause execution forks!
// Only modify this test if the hash calculation is expected to change.
func TestCalculateExecutionDataRootID(t *testing.T) {
	t.Parallel()

	// ONLY modify this hash if it was expected to change. Unintentional changes may cause execution forks!
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

// TestCalculateChunkExecutionDataID tests that CalculateChunkExecutionDataID calculates the correct
// ID given a static ChunkExecutionData. This is used to ensure library updates or modification to
// the provider do not change the ID calculation logic.
//
// CAUTION: Unintentional changes may cause execution forks!
// Only modify this test if the hash calculation is expected to change.
func TestCalculateChunkExecutionDataID(t *testing.T) {
	t.Parallel()

	rootHash, err := ledger.ToRootHash([]byte("0123456789acbdef0123456789acbdef"))
	require.NoError(t, err)

	// ONLY modify this hash if it was expected to change. Unintentional changes may cause execution forks!
	expected := cid.MustParse("QmSZ4sMzj8Be7kkZekjHKppmx2os87oAHV87WFUgZTMrWf")

	ced := execution_data.ChunkExecutionData{
		Collection: &flow.Collection{
			Transactions: []*flow.TransactionBody{
				{Script: []byte("access(all) fun main() {}")},
			},
		},
		Events: []flow.Event{
			unittest.EventFixture(
				unittest.Event.WithEventType("A.0123456789abcdef.SomeContract.SomeEvent"),
				unittest.Event.WithTransactionIndex(1),
				unittest.Event.WithEventIndex(2),
				unittest.Event.WithTransactionID(flow.MustHexStringToIdentifier("95e0929839063afbe334a3d175bea0775cdf5d93f64299e369d16ce21aa423d3")),
				// do not care about Payload
				unittest.Event.WithPayload([]byte{}),
			),
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

	assert.Equal(t,
		expected, actual,
		"expected and actual CID do not match: expected %s, actual %s",
		expected,
		actual,
	)
}

// TestCalculateExecutionDataLifecycle tests that the execution data is reproduced correctly
// at different stages of the lifecycle. This ensures that the data remains consistent, and
// the hashing logic is correct.
//
// CAUTION: Unintentional changes may cause execution forks!
func TestCalculateExecutionDataLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	bed, bedRoot := canonicalBlockExecutionData(t)

	unittest.RunWithTempDir(t, func(dbDir string) {
		pebbleDir := filepath.Join(dbDir, "pebble")

		t.Run("pebble provider generates correct ID", func(t *testing.T) {
			dsManager, err := edstorage.NewPebbleDatastoreManager(unittest.Logger(), pebbleDir, nil)
			require.NoError(t, err)
			defer dsManager.Close()

			provider := getProvider(getBlobservice(t, dsManager.Datastore()))
			executionDataID, executionDataRoot, err := provider.Provide(ctx, 0, bed)
			require.NoError(t, err)

			assert.Equal(t, canonicalExecutionDataID, executionDataID)
			assert.Equal(t, bedRoot, executionDataRoot)
		})

		t.Run("pebble provider retrieves correct execution data", func(t *testing.T) {
			dsManager, err := edstorage.NewPebbleDatastoreManager(unittest.Logger(), pebbleDir, nil)
			require.NoError(t, err)
			defer dsManager.Close()

			bs := blobs.NewBlobstore(dsManager.Datastore())
			bs.HashOnRead(true) // ensure data read from db matches expected hash
			executionDataStore := execution_data.NewExecutionDataStore(bs, execution_data.DefaultSerializer)

			executionData, err := executionDataStore.Get(ctx, canonicalExecutionDataID)
			require.NoError(t, err)

			deepEqual(t, bed, executionData)
		})
	})

	// test that the data is unchanged after protobuf conversions
	t.Run("grpc proto messages", func(t *testing.T) {
		protoMsg, err := convert.BlockExecutionDataToMessage(bed)
		require.NoError(t, err)

		executionData, err := convert.MessageToBlockExecutionData(protoMsg, flow.Emulator.Chain())
		require.NoError(t, err)

		deepEqual(t, bed, executionData)
	})
}

// canonicalBlockExecutionData returns a block execution data fixture generated using a static random seed.
// this ensures it produces the same data on every run, allowing for deterministic testing of output hashes.
func canonicalBlockExecutionData(t *testing.T) (*execution_data.BlockExecutionData, *flow.BlockExecutionDataRoot) {
	suite := fixtures.NewGeneratorSuite(fixtures.WithSeed(canonicalGeneratorSeed))

	bedGen := suite.BlockExecutionDatas()
	cedGen := suite.ChunkExecutionDatas()
	executionData := bedGen.Fixture(bedGen.WithChunkExecutionDatas(cedGen.List(4)...))

	// use in-memory provider to generate the ExecutionDataRoot and ExecutionDataID
	prov := getProvider(getBlobservice(t, getDatastore()))
	executionDataID, executionDataRoot, err := prov.Provide(context.Background(), 0, executionData)
	require.NoError(t, err)

	// ensure the generated execution data matches the expected canonical value
	// if this fails, then something has change in either the execution data generation or the
	// generator suite.
	require.Equal(t, canonicalExecutionDataID, executionDataID)

	return executionData, executionDataRoot
}

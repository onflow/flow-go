package backend

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/subscription"
	trackermock "github.com/onflow/flow-go/engine/access/subscription/tracker/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ExecutionDataProviderSuite struct {
	suite.Suite

	state   *protocolmock.State
	headers *storagemock.Headers
	seals   *storagemock.Seals
	results *storagemock.ExecutionResults

	tracker *trackermock.ExecutionDataTracker
	edStore execution_data.ExecutionDataStore
	edCache *cache.ExecutionDataCache
}

func TestExecutionDataProviderSuite(t *testing.T) {
	suite.Run(t, new(ExecutionDataProviderSuite))
}

func (s *ExecutionDataProviderSuite) SetupTest() {
	s.state = protocolmock.NewState(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.seals = storagemock.NewSeals(s.T())
	s.results = storagemock.NewExecutionResults(s.T())
	s.tracker = trackermock.NewExecutionDataTracker(s.T())

	// mock protocol params to satisfy provider constructor calls
	params := protocolmock.NewParams(s.T())
	params.On("SporkRootBlockHeight").Return(uint64(0)).Maybe()
	s.state.On("Params").Return(params).Maybe()

	bs := blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	s.edStore = execution_data.NewExecutionDataStore(bs, execution_data.DefaultSerializer)

	hero := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, unittest.Logger(), metrics.NewNoopCollector())
	s.edCache = cache.NewExecutionDataCache(s.edStore, s.headers, s.seals, s.results, hero)
}

// Test that we fail fast if height is greater than highest height according to the tracker
func (s *ExecutionDataProviderSuite) TestEarlyNotReady() {
	ctx := context.Background()

	s.tracker.
		On("GetHighestHeight").
		Return(uint64(9)).
		Once()

	provider := NewExecutionDataProvider(s.tracker, s.edCache, s.state)

	entity, err := provider.ExecutionDataByBlockHeight(ctx, 10)
	s.Require().Error(err)
	s.Nil(entity)
	// ensure the error signals not-ready
	s.ErrorIs(err, subscription.ErrBlockNotReady)
}

// Test successful retrieval of execution data via cache
func (s *ExecutionDataProviderSuite) TestSuccess() {
	ctx := context.Background()

	// Build a single block and corresponding execution data
	block := unittest.BlockFixture()
	seal := unittest.BlockSealsFixture(1)[0]
	result := unittest.ExecutionResultFixture()

	// Create an execution data blob for this block in shared store
	ed := unittest.BlockExecutionDataFixture(
		unittest.WithBlockExecutionDataBlockID(block.ID()),
	)
	var err error
	result.ExecutionDataID, err = s.edStore.Add(ctx, ed)
	s.Require().NoError(err)

	seal.ResultID = result.ID()

	// Set expectations on shared storage mocks
	s.headers.
		On("BlockIDByHeight", block.Height).
		Return(block.ID(), nil).
		Once()
	s.seals.
		On("FinalizedSealForBlock", block.ID()).
		Return(seal, nil).
		Once()
	s.results.
		On("ByID", mock.Anything).
		Return(result, nil).
		Once()
	s.tracker.
		On("GetHighestHeight").
		Return(block.Height).
		Once()

	provider := NewExecutionDataProvider(s.tracker, s.edCache, s.state)

	entity, err := provider.ExecutionDataByBlockHeight(ctx, block.Height)
	s.Require().NoError(err)
	s.Require().NotNil(entity)
	s.Equal(block.ID(), entity.BlockID)
}

// Test that not-found scenarios from the cache are mapped to ErrBlockNotReady
func (s *ExecutionDataProviderSuite) TestNotFoundMappedToNotReady() {
	ctx := context.Background()
	height := uint64(100)

	s.tracker.
		On("GetHighestHeight").
		Return(height).
		Once()

	// Make headers return storage.ErrNotFound to simulate missing mapping
	s.headers.
		On("BlockIDByHeight", height).
		Return(flow.ZeroID, storage.ErrNotFound).
		Once()

	provider := NewExecutionDataProvider(s.tracker, s.edCache, s.state)

	entity, err := provider.ExecutionDataByBlockHeight(ctx, height)
	s.Require().Error(err)
	s.Nil(entity)
	s.ErrorIs(err, subscription.ErrBlockNotReady)
}

// Test that blob-not-found errors from backend are mapped to ErrBlockNotReady
func (s *ExecutionDataProviderSuite) TestBlobNotFoundMappedToNotReady() {
	ctx := context.Background()
	height := uint64(42)

	s.tracker.
		On("GetHighestHeight").
		Return(height).
		Once()

	// We will have headers resolve a block ID, then make seals/results work,
	// but make the backend fail with a blob-not-found by using a store that does not have the data
	block := unittest.BlockFixture()

	s.headers.
		On("BlockIDByHeight", height).
		Return(block.ID(), nil).
		Once()

	// Make seals/results return a result that points to a non-existent execution data ID
	missingExecDataID := unittest.IdentifierFixture()
	seal := unittest.BlockSealsFixture(1)[0]
	result := unittest.ExecutionResultFixture()
	seal.ResultID = result.ID()
	result.ExecutionDataID = missingExecDataID

	s.seals.
		On("FinalizedSealForBlock", block.ID()).
		Return(seal, nil).
		Once()
	s.results.
		On("ByID", mock.Anything).
		Return(result, nil).
		Once()

	provider := NewExecutionDataProvider(s.tracker, s.edCache, s.state)

	entity, err := provider.ExecutionDataByBlockHeight(ctx, height)
	s.Require().Error(err)
	s.Nil(entity)
	// ensure the error is recognized as not-ready and also a BlobNotFoundError
	s.ErrorIs(err, subscription.ErrBlockNotReady)
	var blobNotFound *execution_data.BlobNotFoundError
	s.ErrorAs(err, &blobNotFound)
}

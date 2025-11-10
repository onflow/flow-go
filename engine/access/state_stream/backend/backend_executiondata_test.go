package backend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	trackermock "github.com/onflow/flow-go/engine/access/subscription/mock"
	"github.com/onflow/flow-go/engine/access/subscription/streamer"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	osyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

var (
	chainID        = flow.MonotonicEmulator
	testEventTypes = []flow.EventType{
		unittest.EventTypeFixture(chainID),
		unittest.EventTypeFixture(chainID),
		unittest.EventTypeFixture(chainID),
	}
)

type BackendExecutionDataSuite struct {
	suite.Suite
	logger         zerolog.Logger
	state          *protocolmock.State
	params         *protocolmock.Params
	snapshot       *protocolmock.Snapshot
	headers        *storagemock.Headers
	events         *storagemock.Events
	seals          *storagemock.Seals
	results        *storagemock.ExecutionResults
	registers      *storagemock.RegisterIndex
	registersAsync *execution.RegistersAsyncStore
	eventsIndex    *index.EventsIndex

	bs                       blobs.Blobstore
	eds                      execution_data.ExecutionDataStore
	broadcaster              *engine.Broadcaster
	execDataCache            *cache.ExecutionDataCache
	execDataHeroCache        *herocache.BlockExecutionData
	executionDataTracker     *trackermock.ExecutionDataTracker
	backend                  *StateStreamBackend
	executionDataTrackerReal subscription.ExecutionDataTracker

	executionResultProvider *osyncmock.ExecutionResultProvider
	executionStateCache     *osyncmock.ExecutionStateCache
	executionDataSnapshot   *osyncmock.Snapshot
	criteria                optimistic_sync.Criteria

	blocks      []*flow.Block
	blockEvents map[flow.Identifier][]flow.Event
	execDataMap map[flow.Identifier]*execution_data.BlockExecutionDataEntity
	blockMap    map[uint64]*flow.Block
	sealMap     map[flow.Identifier]*flow.Seal
	resultMap   map[flow.Identifier]*flow.ExecutionResult
	registerID  flow.RegisterID

	rootBlock          *flow.Block
	highestBlockHeader *flow.Header
}

type executionDataTestType struct {
	name            string
	highestBackfill int
	startBlockID    flow.Identifier
	startHeight     uint64
}

func TestBackendExecutionDataSuite(t *testing.T) {
	suite.Run(t, new(BackendExecutionDataSuite))
}

func (s *BackendExecutionDataSuite) SetupTest() {
	blockCount := 5
	s.SetupTestSuite(blockCount)

	var err error
	parent := s.rootBlock.ToHeader()

	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.ToHeader()

		seal := unittest.BlockSealsFixture(1)[0]
		result := unittest.ExecutionResultFixture()
		blockEvents := generateMockEvents(block.ToHeader(), (i%len(testEventTypes))*3+1)

		numChunks := 5
		chunkDatas := make([]*execution_data.ChunkExecutionData, 0, numChunks)
		for i := 0; i < numChunks; i++ {
			var events flow.EventsList
			switch {
			case i >= len(blockEvents.Events):
				events = flow.EventsList{}
			case i == numChunks-1:
				events = blockEvents.Events[i:]
			default:
				events = flow.EventsList{blockEvents.Events[i]}
			}
			chunkDatas = append(chunkDatas, unittest.ChunkExecutionDataFixture(s.T(), execution_data.DefaultMaxBlobSize/5, unittest.WithChunkEvents(events)))
		}
		execData := unittest.BlockExecutionDataFixture(
			unittest.WithBlockExecutionDataBlockID(block.ID()),
			unittest.WithChunkExecutionDatas(chunkDatas...),
		)

		result.ExecutionDataID, err = s.eds.Add(context.TODO(), execData)
		assert.NoError(s.T(), err)

		s.blocks = append(s.blocks, block)
		s.execDataMap[block.ID()] = execution_data.NewBlockExecutionDataEntity(result.ExecutionDataID, execData)
		s.blockEvents[block.ID()] = blockEvents.Events
		s.blockMap[block.Height] = block
		s.sealMap[block.ID()] = seal
		s.resultMap[seal.ResultID] = result

		s.T().Logf("adding exec data for block %d %d %v => %v", i, block.Height, block.ID(), result.ExecutionDataID)
	}

	s.SetupTestMocks()
}

func (s *BackendExecutionDataSuite) SetupTestSuite(blockCount int) {
	s.logger = unittest.Logger()

	s.state = protocolmock.NewState(s.T())
	s.snapshot = protocolmock.NewSnapshot(s.T())
	s.params = protocolmock.NewParams(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.events = storagemock.NewEvents(s.T())
	s.seals = storagemock.NewSeals(s.T())
	s.results = storagemock.NewExecutionResults(s.T())

	s.bs = blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	s.eds = execution_data.NewExecutionDataStore(s.bs, execution_data.DefaultSerializer)

	s.broadcaster = engine.NewBroadcaster()

	s.execDataHeroCache = herocache.NewBlockExecutionData(subscription.DefaultCacheSize, s.logger, metrics.NewNoopCollector())
	s.execDataCache = cache.NewExecutionDataCache(s.eds, s.headers, s.seals, s.results, s.execDataHeroCache)
	s.executionDataTracker = trackermock.NewExecutionDataTracker(s.T())

	s.execDataMap = make(map[flow.Identifier]*execution_data.BlockExecutionDataEntity, blockCount)
	s.blockEvents = make(map[flow.Identifier][]flow.Event, blockCount)
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.sealMap = make(map[flow.Identifier]*flow.Seal, blockCount)
	s.resultMap = make(map[flow.Identifier]*flow.ExecutionResult, blockCount)
	s.blocks = make([]*flow.Block, 0, blockCount)

	s.executionDataSnapshot = osyncmock.NewSnapshot(s.T())
	s.executionResultProvider = osyncmock.NewExecutionResultProvider(s.T())
	s.executionStateCache = osyncmock.NewExecutionStateCache(s.T())
	s.criteria = optimistic_sync.Criteria{}

	// generate blockCount consecutive blocks with associated seal, result and execution data
	s.rootBlock = unittest.BlockFixture()
	s.blockMap[s.rootBlock.Height] = s.rootBlock
	s.highestBlockHeader = s.rootBlock.ToHeader()

	s.T().Logf("Generating %d blocks, root block: %d %s", blockCount, s.rootBlock.Height, s.rootBlock.ID())
}

func (s *BackendExecutionDataSuite) SetupTestMocks() {
	s.registerID = unittest.RegisterIDFixture()

	s.eventsIndex = index.NewEventsIndex(index.NewReporter(), s.events)
	s.registersAsync = execution.NewRegistersAsyncStore()
	s.registers = storagemock.NewRegisterIndex(s.T())
	err := s.registersAsync.Initialize(s.registers)
	require.NoError(s.T(), err)
	s.registers.On("LatestHeight").Return(s.rootBlock.Height).Maybe()
	s.registers.On("FirstHeight").Return(s.rootBlock.Height).Maybe()
	s.registers.On("Get", mock.AnythingOfType("RegisterID"), mock.AnythingOfType("uint64")).Return(
		func(id flow.RegisterID, height uint64) (flow.RegisterValue, error) {
			if id == s.registerID {
				return flow.RegisterValue{}, nil
			}
			return nil, storage.ErrNotFound
		}).Maybe()

	s.state.On("Sealed").Return(s.snapshot, nil).Maybe()
	s.snapshot.On("Head").Return(s.blocks[0].ToHeader(), nil).Maybe()

	s.state.On("Params").Return(s.params).Maybe()
	s.params.On("SporkRootBlockHeight").Return(s.rootBlock.Height, nil).Maybe()
	s.params.On("SporkRootBlock").Return(s.rootBlock, nil).Maybe()
	s.headers.On("BlockIDByHeight", s.rootBlock.Height).Return(s.rootBlock.ID(), nil).Maybe()

	s.seals.On("FinalizedSealForBlock", mock.AnythingOfType("flow.Identifier")).Return(
		mocks.StorageMapGetter(s.sealMap),
	).Maybe()

	s.results.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
		mocks.StorageMapGetter(s.resultMap),
	).Maybe()

	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) (*flow.Header, error) {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.ToHeader(), nil
				}
			}
			return nil, storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.ConvertStorageOutput(
			mocks.StorageMapGetter(s.blockMap),
			func(block *flow.Block) *flow.Header { return block.ToHeader() },
		),
	).Maybe()

	s.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.ConvertStorageOutput(
			mocks.StorageMapGetter(s.blockMap),
			func(block *flow.Block) flow.Identifier { return block.ID() },
		),
	).Maybe()

	s.SetupBackend(false)
}

func (s *BackendExecutionDataSuite) SetupBackend(useEventsIndex bool) {
	var err error
	s.backend, err = New(
		s.logger,
		s.state,
		s.headers,
		s.seals,
		s.results,
		s.eds,
		s.execDataCache,
		s.registersAsync,
		s.eventsIndex,
		useEventsIndex,
		state_stream.DefaultRegisterIDsRequestLimit,
		s.executionDataTracker,
		s.executionResultProvider,
		s.executionStateCache,
		s.broadcaster,
		streamer.NewDefaultStreamOptions(),
	)
	require.NoError(s.T(), err)

	// create real execution data tracker to use GetStartHeight from it, instead of mocking
	s.executionDataTrackerReal = tracker.NewExecutionDataTracker(
		s.logger,
		s.state,
		s.rootBlock.Height,
		s.headers,
		s.broadcaster,
		s.rootBlock.Height,
		s.eventsIndex,
		useEventsIndex,
	)

	s.executionDataTracker.On(
		"GetStartHeight",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeight(ctx, startBlockID, startHeight)
	}, nil).Maybe()

	s.executionDataTracker.On("GetHighestHeight").Return(func() uint64 {
		return s.highestBlockHeader.Height
	}).Maybe()
}

// generateMockEvents generates a set of mock events for a block split into multiple tx with
// appropriate indexes set
func generateMockEvents(header *flow.Header, eventCount int) flow.BlockEvents {
	txCount := eventCount / 3

	txID := unittest.IdentifierFixture()
	txIndex := uint32(0)
	eventIndex := uint32(0)

	events := make([]flow.Event, eventCount)
	for i := 0; i < eventCount; i++ {
		if i > 0 && i%txCount == 0 {
			txIndex++
			txID = unittest.IdentifierFixture()
			eventIndex = 0
		}

		events[i] = unittest.EventFixture(
			unittest.Event.WithEventType(testEventTypes[i%len(testEventTypes)]),
			unittest.Event.WithTransactionIndex(txIndex),
			unittest.Event.WithEventIndex(eventIndex),
			unittest.Event.WithTransactionID(txID),
		)
	}

	return flow.BlockEvents{
		BlockID:        header.ID(),
		BlockHeight:    header.Height,
		BlockTimestamp: time.UnixMilli(int64(header.Timestamp)).UTC(),
		Events:         events,
	}
}

func (s *BackendExecutionDataSuite) TestGetExecutionDataByBlockID() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	block := s.blocks[0]
	seal := s.sealMap[block.ID()]
	result := s.resultMap[seal.ResultID]
	execData := s.execDataMap[block.ID()]

	// notify backend block is available
	s.highestBlockHeader = block.ToHeader()

	executionNodes := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))

	var err error
	reader := osyncmock.NewBlockExecutionDataReader(s.T())
	s.Run("happy path TestGetExecutionDataByBlockID success", func() {
		result.ExecutionDataID, err = s.eds.Add(ctx, execData.BlockExecutionData)
		require.NoError(s.T(), err)

		metadata := &accessmodel.ExecutorMetadata{
			ExecutionResultID: result.ID(),
			ExecutorIDs:       executionNodes.NodeIDs(),
		}

		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(&optimistic_sync.ExecutionResultInfo{
				ExecutionResultID: result.ID(),
				ExecutionNodes:    executionNodes.ToSkeleton(),
			}, nil).
			Once()

		s.executionStateCache.
			On("Snapshot", result.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()

		s.executionDataSnapshot.
			On("BlockExecutionData").
			Return(reader).
			Once()

		reader.
			On("ByBlockID", mock.Anything, block.ID()).
			Return(execData, nil).
			Once()

		res, resMetadata, err := s.backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.NotNil(s.T(), resMetadata)
		assert.Equal(s.T(), metadata, resMetadata)
		assert.Equal(s.T(), execData.BlockExecutionData, res)
		assert.NoError(s.T(), err)
	})

	s.execDataHeroCache.Clear()

	s.Run("execution result info returns data not found", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(nil, storage.ErrNotFound).
			Once()

		execDataRes, metadata, err := s.backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		require.Error(s.T(), err)
		require.True(s.T(), access.IsDataNotFoundError(err))
	})

	s.Run("execution result info returns unexpected error", func() {
		expectedErr := fmt.Errorf("failed to get execution result info for block: %w", storage.ErrDataMismatch)
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(nil, storage.ErrDataMismatch).
			Once()

		ctxSignaler := irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, expectedErr)
		ctxIrr := irrecoverable.WithSignalerContext(ctx, ctxSignaler)

		execDataRes, metadata, err := s.backend.GetExecutionDataByBlockID(ctxIrr, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		assert.Error(s.T(), err)
	})

	s.Run("snapshot returns data not found", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(&optimistic_sync.ExecutionResultInfo{
				ExecutionResultID: result.ID(),
				ExecutionNodes:    executionNodes.ToSkeleton(),
			}, nil).
			Once()

		s.executionStateCache.
			On("Snapshot", result.ID()).
			Return(nil, storage.ErrNotFound).
			Once()

		execDataRes, metadata, err := s.backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		require.Error(s.T(), err)
		require.True(s.T(), access.IsDataNotFoundError(err))
	})

	s.Run("snapshot returns unexpected error", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(&optimistic_sync.ExecutionResultInfo{
				ExecutionResultID: result.ID(),
				ExecutionNodes:    executionNodes.ToSkeleton(),
			}, nil).
			Once()

		expectedError := fmt.Errorf("unexpected error")
		s.executionStateCache.
			On("Snapshot", result.ID()).
			Return(nil, expectedError).
			Once()

		ctxSignaler := irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, expectedError)
		ctxIrr := irrecoverable.WithSignalerContext(ctx, ctxSignaler)

		execDataRes, metadata, err := s.backend.GetExecutionDataByBlockID(ctxIrr, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		assert.Error(s.T(), err)
	})

	s.Run("missing exec data for TestGetExecutionDataByBlockID failure", func() {
		result.ExecutionDataID = unittest.IdentifierFixture()

		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(&optimistic_sync.ExecutionResultInfo{
				ExecutionResultID: result.ID(),
				ExecutionNodes:    executionNodes.ToSkeleton(),
			}, nil).
			Once()

		s.executionStateCache.
			On("Snapshot", result.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()

		s.executionDataSnapshot.
			On("BlockExecutionData").
			Return(reader).
			Once()

		reader.
			On("ByBlockID", mock.Anything, block.ID()).
			Return(nil, storage.ErrNotFound).
			Once()

		execDataRes, metadata, err := s.backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		s.Require().True(access.IsDataNotFoundError(err))
	})

	s.Run("unexpected error from ByBlockID", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(&optimistic_sync.ExecutionResultInfo{
				ExecutionResultID: result.ID(),
				ExecutionNodes:    executionNodes.ToSkeleton(),
			}, nil).
			Once()

		s.executionStateCache.
			On("Snapshot", result.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()

		s.executionDataSnapshot.
			On("BlockExecutionData").
			Return(reader).
			Once()

		reader.
			On("ByBlockID", mock.Anything, block.ID()).
			Return(nil, storage.ErrDataMismatch).
			Once()

		expectedError := fmt.Errorf("unexpected error getting execution data: %w", storage.ErrDataMismatch)
		ctxSignaler := irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, expectedError)
		ctxIrr := irrecoverable.WithSignalerContext(ctx, ctxSignaler)

		execDataRes, metadata, err := s.backend.GetExecutionDataByBlockID(ctxIrr, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		assert.Error(s.T(), err)
	})
}

func (s *BackendExecutionDataSuite) TestSubscribeExecutionData() {
	tests := []executionDataTestType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startBlockID:    flow.ZeroID,
			startHeight:     0,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startBlockID:    flow.ZeroID,
			startHeight:     s.blocks[0].Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startBlockID:    s.blocks[0].ID(),
			startHeight:     0,
		},
	}

	subFunc := func(
		ctx context.Context,
		blockID flow.Identifier,
		startHeight uint64,
	) subscription.Subscription[*state_stream.ExecutionDataResponse] {
		return s.backend.SubscribeExecutionData(ctx, blockID, startHeight)
	}

	subscribeExecData(s, subFunc, tests)
}

func (s *BackendExecutionDataSuite) TestSubscribeExecutionDataFromStartBlockID() {
	tests := []executionDataTestType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startBlockID:    s.blocks[0].ID(),
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startBlockID:    s.blocks[0].ID(),
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startBlockID:    s.blocks[0].ID(),
		},
	}

	s.executionDataTracker.On(
		"GetStartHeightFromBlockID",
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	subFunc := func(
		ctx context.Context,
		blockID flow.Identifier,
		startHeight uint64,
	) subscription.Subscription[*state_stream.ExecutionDataResponse] {
		return s.backend.SubscribeExecutionDataFromStartBlockID(ctx, blockID)
	}

	subscribeExecData(s, subFunc, tests)
}

func (s *BackendExecutionDataSuite) TestSubscribeExecutionDataFromStartBlockHeight() {
	tests := []executionDataTestType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startHeight:     s.blocks[0].Height,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startHeight:     s.blocks[0].Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startHeight:     s.blocks[0].Height,
		},
	}

	s.executionDataTracker.On(
		"GetStartHeightFromHeight",
		mock.AnythingOfType("uint64"),
	).Return(func(startHeight uint64) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
	}, nil)

	subFunc := func(
		ctx context.Context,
		blockID flow.Identifier,
		startHeight uint64,
	) subscription.Subscription[*state_stream.ExecutionDataResponse] {
		return s.backend.SubscribeExecutionDataFromStartBlockHeight(ctx, startHeight)
	}

	subscribeExecData(s, subFunc, tests)
}

func (s *BackendExecutionDataSuite) TestSubscribeExecutionDataFromLatest() {
	tests := []executionDataTestType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
		},
	}

	s.executionDataTracker.On(
		"GetStartHeightFromLatest",
		mock.Anything,
	).Return(func(ctx context.Context) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromLatest(ctx)
	}, nil)

	subFunc := func(
		ctx context.Context,
		blockID flow.Identifier,
		startHeight uint64,
	) subscription.Subscription[*state_stream.ExecutionDataResponse] {
		return s.backend.SubscribeExecutionDataFromLatest(ctx)
	}

	subscribeExecData(s, subFunc, tests)
}

func subscribeExecData(
	s *BackendExecutionDataSuite,
	subscribeFunc func(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) subscription.Subscription[*state_stream.ExecutionDataResponse],
	tests []executionDataTestType,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, test := range tests {
		s.Run(test.name, func() {
			// make sure we're starting with a fresh cache
			s.execDataHeroCache.Clear()

			s.T().Logf("len(s.execDataMap) %d", len(s.execDataMap))

			// add "backfill" block - blocks that are already in the database before the test starts
			// this simulates a subscription on a past block
			for i := 0; i <= test.highestBackfill; i++ {
				s.T().Logf("backfilling block %d", i)
				s.highestBlockHeader = s.blocks[i].ToHeader()
			}

			subCtx, subCancel := context.WithCancel(ctx)
			sub := subscribeFunc(subCtx, test.startBlockID, test.startHeight)

			// loop over of the all blocks
			for i, b := range s.blocks {
				execData := s.execDataMap[b.ID()]
				s.T().Logf("checking block %d %v %v", i, b.Height, b.ID())

				// simulate new exec data received.
				// exec data for all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.highestBlockHeader = b.ToHeader()
					s.broadcaster.Publish()
				}

				// consume execution data from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					resp, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v: err: %v", b.Height, b.ID(), sub.Err())
					assert.Equal(s.T(), b.Height, resp.Height)
					assert.Equal(s.T(), execData.BlockExecutionData, resp.ExecutionData)
				}, time.Second, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Height, b.ID()))
			}

			// make sure there are no new messages waiting. the channel should be opened with nothing waiting
			unittest.RequireNeverReturnBefore(s.T(), func() {
				<-sub.Channel()
			}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")

			// stop the subscription
			subCancel()

			// ensure subscription shuts down gracefully
			unittest.RequireReturnsBefore(s.T(), func() {
				v, ok := <-sub.Channel()
				assert.Nil(s.T(), v)
				assert.False(s.T(), ok)
				assert.ErrorIs(s.T(), sub.Err(), context.Canceled)
			}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")
		})
	}
}

// TestSubscribeEventsFromSporkRootBlock tests that events subscriptions starting from the spork
// root block return an empty result for the root block.
func (s *BackendExecutionDataSuite) TestSubscribeExecutionFromSporkRootBlock() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// setup the backend to have 1 available block
	s.highestBlockHeader = s.blocks[0].ToHeader()

	rootEventResponse := &state_stream.ExecutionDataResponse{
		Height: s.rootBlock.Height,
		ExecutionData: &execution_data.BlockExecutionData{
			BlockID: s.rootBlock.ID(),
		},
	}

	firstEventResponse := &state_stream.ExecutionDataResponse{
		Height:        s.blocks[0].Height,
		ExecutionData: s.execDataMap[s.blocks[0].ID()].BlockExecutionData,
	}

	assertExecutionDataResponse := func(v interface{}, expected *state_stream.ExecutionDataResponse) {
		resp, ok := v.(*state_stream.ExecutionDataResponse)
		require.True(s.T(), ok, "unexpected response type: %T", v)

		assert.Equal(s.T(), expected, resp)
	}

	assertSubscriptionResponses := func(sub subscription.Subscription[*state_stream.ExecutionDataResponse], cancel context.CancelFunc) {
		// the first response should have details from the root block and no events
		resp := <-sub.Channel()
		assertExecutionDataResponse(resp, rootEventResponse)

		// the second response should have details from the first block and its events
		resp = <-sub.Channel()
		assertExecutionDataResponse(resp, firstEventResponse)

		cancel()
		resp, ok := <-sub.Channel()
		assert.False(s.T(), ok)
		assert.Nil(s.T(), resp)
		assert.ErrorIs(s.T(), sub.Err(), context.Canceled)
	}

	s.Run("by height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromHeight", s.rootBlock.Height).
			Return(func(startHeight uint64) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
			})

		sub := s.backend.SubscribeExecutionDataFromStartBlockHeight(subCtx, s.rootBlock.Height)
		assertSubscriptionResponses(sub, subCancel)
	})

	s.Run("by height - legacy", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromHeight", s.rootBlock.Height).
			Return(func(startHeight uint64) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
			})

		sub := s.backend.SubscribeExecutionData(subCtx, flow.ZeroID, s.rootBlock.Height)
		assertSubscriptionResponses(sub, subCancel)
	})

	s.Run("by ID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromBlockID", s.rootBlock.ID()).
			Return(func(startBlockID flow.Identifier) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeightFromBlockID(startBlockID)
			})

		sub := s.backend.SubscribeExecutionDataFromStartBlockID(subCtx, s.rootBlock.ID())
		assertSubscriptionResponses(sub, subCancel)
	})

	s.Run("by ID - legacy", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromBlockID", s.rootBlock.ID()).
			Return(func(startBlockID flow.Identifier) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeightFromBlockID(startBlockID)
			})

		sub := s.backend.SubscribeExecutionData(subCtx, s.rootBlock.ID(), 0)
		assertSubscriptionResponses(sub, subCancel)
	})

	s.Run("by latest", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		// simulate the case where the latest block is also the root block
		s.snapshot.On("Head").Unset()
		s.snapshot.On("Head").Return(s.rootBlock.ToHeader(), nil).Once()

		s.executionDataTracker.On("GetStartHeightFromLatest", mock.Anything).
			Return(func(ctx context.Context) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeightFromLatest(ctx)
			})

		sub := s.backend.SubscribeExecutionDataFromLatest(subCtx)
		assertSubscriptionResponses(sub, subCancel)
	})

	s.Run("by latest - legacy", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		// simulate the case where the latest block is also the root block
		s.snapshot.On("Head").Unset()
		s.snapshot.On("Head").Return(s.rootBlock.ToHeader(), nil).Once()

		s.executionDataTracker.On("GetStartHeightFromLatest", mock.Anything).
			Return(func(ctx context.Context) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeightFromLatest(ctx)
			})

		sub := s.backend.SubscribeExecutionData(subCtx, flow.ZeroID, 0)
		assertSubscriptionResponses(sub, subCancel)
	})
}

func (s *BackendExecutionDataSuite) TestSubscribeExecutionDataHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Run("returns error if both start blockID and start height are provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeExecutionData(subCtx, unittest.IdentifierFixture(), 1)
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()))
	})

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeExecutionData(subCtx, flow.ZeroID, s.rootBlock.Height-1)
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()))
	})

	s.Run("returns error for unindexed start blockID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeExecutionData(subCtx, unittest.IdentifierFixture(), 0)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()))
	})

	// make sure we're starting with a fresh cache
	s.execDataHeroCache.Clear()

	s.Run("returns error for unindexed start height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeExecutionData(subCtx, flow.ZeroID, s.blocks[len(s.blocks)-1].Height+10)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()))
	})
}

func (s *BackendExecutionDataSuite) TestGetRegisterValues() {
	s.Run("normal case", func() {
		res, err := s.backend.GetRegisterValues(flow.RegisterIDs{s.registerID}, s.rootBlock.Height)
		require.NoError(s.T(), err)
		require.NotEmpty(s.T(), res)
	})

	s.Run("returns error if block height is out of range", func() {
		res, err := s.backend.GetRegisterValues(flow.RegisterIDs{s.registerID}, s.rootBlock.Height+1)
		require.Nil(s.T(), res)
		require.Equal(s.T(), codes.OutOfRange, status.Code(err))
	})

	s.Run("returns error if register path is not indexed", func() {
		falseID := flow.RegisterIDs{flow.RegisterID{Owner: "ha", Key: "ha"}}
		res, err := s.backend.GetRegisterValues(falseID, s.rootBlock.Height)
		require.Nil(s.T(), res)
		require.Equal(s.T(), codes.NotFound, status.Code(err))
	})

	s.Run("returns error if too many registers are requested", func() {
		res, err := s.backend.GetRegisterValues(make(flow.RegisterIDs, s.backend.registerRequestLimit+1), s.rootBlock.Height)
		require.Nil(s.T(), res)
		require.Equal(s.T(), codes.InvalidArgument, status.Code(err))
	})
}

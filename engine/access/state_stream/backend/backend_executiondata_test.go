package backend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

var testEventTypes = []flow.EventType{
	"A.0x1.Foo.Bar",
	"A.0x2.Zoo.Moo",
	"A.0x3.Goo.Hoo",
}

type BackendExecutionDataSuite struct {
	suite.Suite

	state          *protocolmock.State
	params         *protocolmock.Params
	snapshot       *protocolmock.Snapshot
	headers        *storagemock.Headers
	seals          *storagemock.Seals
	results        *storagemock.ExecutionResults
	registers      *storagemock.RegisterIndex
	registersAsync *execution.RegistersAsyncStore

	bs                blobs.Blobstore
	eds               execution_data.ExecutionDataStore
	broadcaster       *engine.Broadcaster
	execDataCache     *cache.ExecutionDataCache
	execDataHeroCache *herocache.BlockExecutionData
	backend           *StateStreamBackend

	blocks      []*flow.Block
	blockEvents map[flow.Identifier]flow.EventsList
	execDataMap map[flow.Identifier]*execution_data.BlockExecutionDataEntity
	blockMap    map[uint64]*flow.Block
	sealMap     map[flow.Identifier]*flow.Seal
	resultMap   map[flow.Identifier]*flow.ExecutionResult
	registerID  flow.RegisterID
}

func TestBackendExecutionDataSuite(t *testing.T) {
	suite.Run(t, new(BackendExecutionDataSuite))
}

func (s *BackendExecutionDataSuite) SetupTest() {
	logger := unittest.Logger()

	s.state = protocolmock.NewState(s.T())
	s.snapshot = protocolmock.NewSnapshot(s.T())
	s.params = protocolmock.NewParams(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.seals = storagemock.NewSeals(s.T())
	s.results = storagemock.NewExecutionResults(s.T())

	s.bs = blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	s.eds = execution_data.NewExecutionDataStore(s.bs, execution_data.DefaultSerializer)

	s.broadcaster = engine.NewBroadcaster()

	s.execDataHeroCache = herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metrics.NewNoopCollector())
	s.execDataCache = cache.NewExecutionDataCache(s.eds, s.headers, s.seals, s.results, s.execDataHeroCache)

	conf := Config{
		ClientSendTimeout:       subscription.DefaultSendTimeout,
		ClientSendBufferSize:    subscription.DefaultSendBufferSize,
		RegisterIDsRequestLimit: state_stream.DefaultRegisterIDsRequestLimit,
	}

	var err error

	blockCount := 5
	s.execDataMap = make(map[flow.Identifier]*execution_data.BlockExecutionDataEntity, blockCount)
	s.blockEvents = make(map[flow.Identifier]flow.EventsList, blockCount)
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.sealMap = make(map[flow.Identifier]*flow.Seal, blockCount)
	s.resultMap = make(map[flow.Identifier]*flow.ExecutionResult, blockCount)
	s.blocks = make([]*flow.Block, 0, blockCount)

	// generate blockCount consecutive blocks with associated seal, result and execution data
	rootBlock := unittest.BlockFixture()
	parent := rootBlock.Header
	s.blockMap[rootBlock.Header.Height] = &rootBlock

	s.T().Logf("Generating %d blocks, root block: %d %s", blockCount, rootBlock.Header.Height, rootBlock.ID())

	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.Header

		seal := unittest.BlockSealsFixture(1)[0]
		result := unittest.ExecutionResultFixture()
		blockEvents := unittest.BlockEventsFixture(block.Header, (i%len(testEventTypes))*3+1, testEventTypes...)

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
		s.blockMap[block.Header.Height] = block
		s.sealMap[block.ID()] = seal
		s.resultMap[seal.ResultID] = result

		s.T().Logf("adding exec data for block %d %d %v => %v", i, block.Header.Height, block.ID(), result.ExecutionDataID)
	}

	s.registerID = unittest.RegisterIDFixture()

	s.registersAsync = execution.NewRegistersAsyncStore()
	s.registers = storagemock.NewRegisterIndex(s.T())
	err = s.registersAsync.InitDataAvailable(s.registers)
	require.NoError(s.T(), err)
	s.registers.On("LatestHeight").Return(rootBlock.Header.Height).Maybe()
	s.registers.On("FirstHeight").Return(rootBlock.Header.Height).Maybe()
	s.registers.On("Get", mock.AnythingOfType("RegisterID"), mock.AnythingOfType("uint64")).Return(
		func(id flow.RegisterID, height uint64) (flow.RegisterValue, error) {
			if id == s.registerID {
				return flow.RegisterValue{}, nil
			}
			return nil, storage.ErrNotFound
		}).Maybe()

	s.state.On("Sealed").Return(s.snapshot, nil).Maybe()
	s.snapshot.On("Head").Return(s.blocks[0].Header, nil).Maybe()

	s.seals.On("FinalizedSealForBlock", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Seal {
			if seal, ok := s.sealMap[blockID]; ok {
				return seal
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			if _, ok := s.sealMap[blockID]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.results.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			if result, ok := s.resultMap[resultID]; ok {
				return result
			}
			return nil
		},
		func(resultID flow.Identifier) error {
			if _, ok := s.resultMap[resultID]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Header {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.Header
				}
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return nil
				}
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) *flow.Header {
			if block, ok := s.blockMap[height]; ok {
				return block.Header
			}
			return nil
		},
		func(height uint64) error {
			if _, ok := s.blockMap[height]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) flow.Identifier {
			if block, ok := s.blockMap[height]; ok {
				return block.Header.ID()
			}
			return flow.ZeroID
		},
		func(height uint64) error {
			if _, ok := s.blockMap[height]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.backend, err = New(
		logger,
		conf,
		s.state,
		s.headers,
		s.seals,
		s.results,
		s.eds,
		s.execDataCache,
		s.broadcaster,
		rootBlock.Header.Height,
		rootBlock.Header.Height, // initialize with no downloaded data
		s.registersAsync,
	)
	require.NoError(s.T(), err)
}

func (s *BackendExecutionDataSuite) TestGetExecutionDataByBlockID() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	block := s.blocks[0]
	seal := s.sealMap[block.ID()]
	result := s.resultMap[seal.ResultID]
	execData := s.execDataMap[block.ID()]

	// notify backend block is available
	s.backend.setHighestHeight(block.Header.Height)

	var err error
	s.Run("happy path TestGetExecutionDataByBlockID success", func() {
		result.ExecutionDataID, err = s.eds.Add(ctx, execData.BlockExecutionData)
		require.NoError(s.T(), err)

		res, err := s.backend.GetExecutionDataByBlockID(ctx, block.ID())
		assert.Equal(s.T(), execData.BlockExecutionData, res)
		assert.NoError(s.T(), err)
	})

	s.execDataHeroCache.Clear()

	s.Run("missing exec data for TestGetExecutionDataByBlockID failure", func() {
		result.ExecutionDataID = unittest.IdentifierFixture()

		execDataRes, err := s.backend.GetExecutionDataByBlockID(ctx, block.ID())
		assert.Nil(s.T(), execDataRes)
		assert.Equal(s.T(), codes.NotFound, status.Code(err))
	})
}

func (s *BackendExecutionDataSuite) TestSubscribeExecutionData() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name            string
		highestBackfill int
		startBlockID    flow.Identifier
		startHeight     uint64
	}{
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
			startHeight:     s.blocks[0].Header.Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startBlockID:    s.blocks[0].ID(),
			startHeight:     0,
		},
		{
			name:            "happy path - start from root block by height",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startBlockID:    flow.ZeroID,
			startHeight:     s.backend.SubscriptionBackendHandler.RootHeight, // start from root block
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1,                                // backfill all blocks
			startBlockID:    s.backend.SubscriptionBackendHandler.RootBlockID, // start from root block
			startHeight:     0,
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			// make sure we're starting with a fresh cache
			s.execDataHeroCache.Clear()

			s.T().Logf("len(s.execDataMap) %d", len(s.execDataMap))

			// add "backfill" block - blocks that are already in the database before the test starts
			// this simulates a subscription on a past block
			for i := 0; i <= test.highestBackfill; i++ {
				s.T().Logf("backfilling block %d", i)
				s.backend.setHighestHeight(s.blocks[i].Header.Height)
			}

			subCtx, subCancel := context.WithCancel(ctx)
			sub := s.backend.SubscribeExecutionData(subCtx, test.startBlockID, test.startHeight)

			// loop over all of the blocks
			for i, b := range s.blocks {
				execData := s.execDataMap[b.ID()]
				s.T().Logf("checking block %d %v", i, b.ID())

				// simulate new exec data received.
				// exec data for all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.backend.setHighestHeight(b.Header.Height)
					s.broadcaster.Publish()
				}

				// consume execution data from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v: err: %v", b.Header.Height, b.ID(), sub.Err())

					resp, ok := v.(*ExecutionDataResponse)
					require.True(s.T(), ok, "unexpected response type: %T", v)

					assert.Equal(s.T(), b.Header.Height, resp.Height)
					assert.Equal(s.T(), execData.BlockExecutionData, resp.ExecutionData)
				}, time.Second, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Header.Height, b.ID()))
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

		sub := s.backend.SubscribeExecutionData(subCtx, flow.ZeroID, s.backend.SubscriptionBackendHandler.RootHeight-1)
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

		sub := s.backend.SubscribeExecutionData(subCtx, flow.ZeroID, s.blocks[len(s.blocks)-1].Header.Height+10)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()))
	})
}

func (s *BackendExecutionDataSuite) TestGetRegisterValues() {
	s.Run("normal case", func() {
		res, err := s.backend.GetRegisterValues(flow.RegisterIDs{s.registerID}, s.backend.SubscriptionBackendHandler.RootHeight)
		require.NoError(s.T(), err)
		require.NotEmpty(s.T(), res)
	})

	s.Run("returns error if block height is out of range", func() {
		_, err := s.backend.GetRegisterValues(flow.RegisterIDs{s.registerID}, s.backend.SubscriptionBackendHandler.RootHeight+1)
		require.Equal(s.T(), codes.OutOfRange, status.Code(err))
	})

	s.Run("returns error if register path is not indexed", func() {
		falseID := flow.RegisterIDs{flow.RegisterID{Owner: "ha", Key: "ha"}}
		_, err := s.backend.GetRegisterValues(falseID, s.backend.SubscriptionBackendHandler.RootHeight)
		require.Equal(s.T(), codes.NotFound, status.Code(err))
	})

	s.Run("returns error if too many registers are requested", func() {
		_, err := s.backend.GetRegisterValues(make(flow.RegisterIDs, s.backend.registerRequestLimit+1), s.backend.SubscriptionBackendHandler.RootHeight)
		require.Equal(s.T(), codes.InvalidArgument, status.Code(err))
	})
}

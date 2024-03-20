package backend

import (
	"context"
	"fmt"
	"testing"
	"time"

	subscriptionmock "github.com/onflow/flow-go/engine/access/subscription/mock"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

var accountAddress = "0x1d7e57aa55817448"

var testProtocolEventTypes = []flow.EventType{
	"flow.AccountCreated",
	"flow.AccountContractAdded",
	"flow.AccountContractUpdated",
}

// Define the test type struct
type testType struct {
	name            string // Test case name
	highestBackfill int    // Highest backfill index
	startValue      interface{}
	filters         state_stream.AccountStatusFilter // Event filters
}

// BackendAccountStatusesSuite is the test suite for AccountStatusesBackend.
type BackendAccountStatusesSuite struct {
	BackendExecutionDataSuite
}

func TestBackendAccountStatusesSuite(t *testing.T) {
	suite.Run(t, new(BackendAccountStatusesSuite))
}

// getAccountCreateEvent returns a mock account creation event.
func (s *BackendAccountStatusesSuite) getAccountCreateEvent(address common.Address) flow.Event {
	cadenceEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewAddress(address),
		}).WithType(&cadence.EventType{
		Location:            stdlib.FlowLocation{},
		QualifiedIdentifier: "AccountCreated",
		Fields: []cadence.Field{
			{
				Identifier: "address",
				Type:       cadence.AddressType{},
			},
		},
	})

	payload, err := ccf.Encode(cadenceEvent)
	require.NoError(s.T(), err)

	event := unittest.EventFixture(
		flow.EventType(cadenceEvent.EventType.Location.TypeID(nil, cadenceEvent.EventType.QualifiedIdentifier)),
		0,
		0,
		unittest.IdentifierFixture(),
		0,
	)

	event.Payload = payload

	return event
}

// getAccountContractEvent returns a mock account contract event.
func (s *BackendAccountStatusesSuite) getAccountContractEvent(qualifiedIdentifier string, address common.Address) flow.Event {
	contractName, err := cadence.NewString("EventContract")
	require.NoError(s.T(), err)

	cadenceEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewAddress(address),
			cadence.NewArray(
				testutils.ConvertToCadence([]byte{111, 43, 164, 202, 220, 174, 148, 17, 253, 161, 9, 124, 237, 83, 227, 75, 115, 149, 141, 83, 129, 145, 252, 68, 122, 137, 80, 155, 89, 233, 136, 213}),
			).WithType(cadence.NewConstantSizedArrayType(32, cadence.TheUInt8Type)),
			contractName,
		}).WithType(&cadence.EventType{
		Location:            stdlib.FlowLocation{},
		QualifiedIdentifier: qualifiedIdentifier,
		Fields: []cadence.Field{
			{
				Identifier: "address",
				Type:       cadence.AddressType{},
			},
			{
				Identifier: "codeHash",
				Type:       cadence.NewConstantSizedArrayType(32, cadence.TheUInt8Type),
			},
			{
				Identifier: "contract",
				Type:       cadence.StringType{},
			},
		},
	})

	payload, err := ccf.Encode(cadenceEvent)
	require.NoError(s.T(), err)

	event := unittest.EventFixture(
		flow.EventType(cadenceEvent.EventType.Location.TypeID(nil, cadenceEvent.EventType.QualifiedIdentifier)),
		0,
		0,
		unittest.IdentifierFixture(),
		0,
	)

	event.Payload = payload

	return event
}

// generateProtocolMockEvents generates a set of mock events.
func (s *BackendAccountStatusesSuite) generateProtocolMockEvents() flow.EventsList {
	events := make([]flow.Event, 4)
	events = append(events, unittest.EventFixture(testEventTypes[0], 0, 0, unittest.IdentifierFixture(), 0))

	address, err := common.HexToAddress(accountAddress)
	require.NoError(s.T(), err)

	accountCreateEvent := s.getAccountCreateEvent(address)
	accountCreateEvent.TransactionIndex = 1
	events = append(events, accountCreateEvent)

	accountContractAdded := s.getAccountContractEvent("AccountContractAdded", address)
	accountContractAdded.TransactionIndex = 2
	events = append(events, accountContractAdded)

	accountContractUpdated := s.getAccountContractEvent("AccountContractUpdated", address)
	accountContractUpdated.TransactionIndex = 3
	events = append(events, accountContractUpdated)

	return events
}

// SetupTest initializes the test suite.
func (s *BackendAccountStatusesSuite) SetupTest() {
	logger := unittest.Logger()

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

	s.execDataHeroCache = herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metrics.NewNoopCollector())
	s.execDataCache = cache.NewExecutionDataCache(s.eds, s.headers, s.seals, s.results, s.execDataHeroCache)
	s.executionDataTracker = subscriptionmock.NewExecutionDataTracker(s.T())

	conf := Config{
		ClientSendTimeout:       subscription.DefaultSendTimeout,
		ClientSendBufferSize:    subscription.DefaultSendBufferSize,
		RegisterIDsRequestLimit: state_stream.DefaultRegisterIDsRequestLimit,
	}

	var err error

	blockCount := 5
	s.execDataMap = make(map[flow.Identifier]*execution_data.BlockExecutionDataEntity, blockCount)
	s.blockEvents = make(map[flow.Identifier][]flow.Event, blockCount)
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.sealMap = make(map[flow.Identifier]*flow.Seal, blockCount)
	s.resultMap = make(map[flow.Identifier]*flow.ExecutionResult, blockCount)
	s.blocks = make([]*flow.Block, 0, blockCount)

	// generate blockCount consecutive blocks with associated seal, result and execution data
	s.rootBlock = unittest.BlockFixture()
	parent := s.rootBlock.Header
	s.blockMap[s.rootBlock.Header.Height] = &s.rootBlock

	s.T().Logf("Generating %d blocks, root block: %d %s", blockCount, s.rootBlock.Header.Height, s.rootBlock.ID())

	events := s.generateProtocolMockEvents()

	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.Header

		seal := unittest.BlockSealsFixture(1)[0]
		result := unittest.ExecutionResultFixture()

		chunkDatas := []*execution_data.ChunkExecutionData{
			unittest.ChunkExecutionDataFixture(s.T(), execution_data.DefaultMaxBlobSize/5, unittest.WithChunkEvents(events)),
		}

		execData := unittest.BlockExecutionDataFixture(
			unittest.WithBlockExecutionDataBlockID(block.ID()),
			unittest.WithChunkExecutionDatas(chunkDatas...),
		)

		result.ExecutionDataID, err = s.eds.Add(context.TODO(), execData)
		assert.NoError(s.T(), err)

		s.blocks = append(s.blocks, block)
		s.execDataMap[block.ID()] = execution_data.NewBlockExecutionDataEntity(result.ExecutionDataID, execData)
		s.blockEvents[block.ID()] = events
		s.blockMap[block.Header.Height] = block
		s.sealMap[block.ID()] = seal
		s.resultMap[seal.ResultID] = result

		s.T().Logf("adding exec data for block %d %d %v => %v", i, block.Header.Height, block.ID(), result.ExecutionDataID)
	}

	s.registerID = unittest.RegisterIDFixture()

	s.eventsIndex = index.NewEventsIndex(s.events)
	s.registersAsync = execution.NewRegistersAsyncStore()
	s.registers = storagemock.NewRegisterIndex(s.T())
	err = s.registersAsync.Initialize(s.registers)
	require.NoError(s.T(), err)
	s.registers.On("LatestHeight").Return(s.rootBlock.Header.Height).Maybe()
	s.registers.On("FirstHeight").Return(s.rootBlock.Header.Height).Maybe()
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
		mocks.StorageMapGetter(s.sealMap),
	).Maybe()

	s.results.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
		mocks.StorageMapGetter(s.resultMap),
	).Maybe()

	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) (*flow.Header, error) {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.Header, nil
				}
			}
			return nil, storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.ConvertStorageOutput(
			mocks.StorageMapGetter(s.blockMap),
			func(block *flow.Block) *flow.Header { return block.Header },
		),
	).Maybe()

	s.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.ConvertStorageOutput(
			mocks.StorageMapGetter(s.blockMap),
			func(block *flow.Block) flow.Identifier { return block.ID() },
		),
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
		s.registersAsync,
		s.eventsIndex,
		false,
		s.executionDataTracker,
	)
	require.NoError(s.T(), err)

	// create real execution data tracker to use GetStartHeight from it, instead of mocking
	s.executionDataTrackerReal = subscription.NewExecutionDataTracker(
		logger,
		s.state,
		s.rootBlock.Header.Height,
		s.headers,
		s.broadcaster,
		s.rootBlock.Header.Height,
		s.eventsIndex,
		false,
	)

	s.executionDataTracker.On(
		"GetStartHeight",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeight(ctx, startBlockID, startHeight)
	}, nil).Maybe()
}

// subscribeFromStartBlockIdTestCases generates test cases for subscribing from a start block ID.
func (s *BackendAccountStatusesSuite) subscribeFromStartBlockIdTestCases() []testType {
	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startValue:      s.rootBlock.ID(),
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startValue:      s.blocks[0].ID(),
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startValue:      s.blocks[0].ID(),
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startValue:      s.rootBlock.ID(),  // start from root block
		},
	}

	return s.generateFiltersForTestCases(baseTests)
}

// subscribeFromStartHeightTestCases generates test cases for subscribing from a start height.
func (s *BackendAccountStatusesSuite) subscribeFromStartHeightTestCases() []testType {
	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startValue:      s.rootBlock.Header.Height,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startValue:      s.blocks[0].Header.Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startValue:      s.blocks[0].Header.Height,
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1,         // backfill all blocks
			startValue:      s.rootBlock.Header.Height, // start from root block
		},
	}

	return s.generateFiltersForTestCases(baseTests)
}

// subscribeFromLatestTestCases generates test cases for subscribing from the latest block.
func (s *BackendAccountStatusesSuite) subscribeFromLatestTestCases() []testType {
	baseTests := []testType{
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

	return s.generateFiltersForTestCases(baseTests)
}

// generateFiltersForTestCases generates variations of test cases with different event filters.
//
// This function takes an array of base testType structs and creates variations for each of them.
// For each base test case, it generates three variations:
// - All events: Includes all protocol event types filtered by the provided account address.
// - Some events: Includes only the first protocol event type filtered by the provided account address.
// - No events: Includes a custom event type "flow.AccountKeyAdded" filtered by the provided account address.
func (s *BackendAccountStatusesSuite) generateFiltersForTestCases(baseTests []testType) []testType {
	// Create variations for each of the base tests
	tests := make([]testType, 0, len(baseTests)*3)
	var err error
	for _, test := range baseTests {
		t1 := test
		t1.name = fmt.Sprintf("%s - all events", test.name)
		t1.filters, err = state_stream.NewAccountStatusFilter(
			state_stream.DefaultEventFilterConfig,
			chainID.Chain(),
			[]string{string(testProtocolEventTypes[0]), string(testProtocolEventTypes[1]), string(testProtocolEventTypes[2])},
			[]string{accountAddress},
		)
		require.NoError(s.T(), err)
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - some events", test.name)
		t2.filters, err = state_stream.NewAccountStatusFilter(
			state_stream.DefaultEventFilterConfig,
			chainID.Chain(),
			[]string{string(testProtocolEventTypes[0])},
			[]string{accountAddress},
		)
		require.NoError(s.T(), err)
		tests = append(tests, t2)

		t3 := test
		t3.name = fmt.Sprintf("%s - no events", test.name)
		t3.filters, err = state_stream.NewAccountStatusFilter(
			state_stream.DefaultEventFilterConfig,
			chainID.Chain(),
			[]string{"flow.AccountKeyAdded"},
			[]string{accountAddress},
		)
		require.NoError(s.T(), err)
		tests = append(tests, t3)
	}

	return tests
}

// subscribeToAccountStatuses runs subscription tests for account statuses.
//
// This function takes a subscribeFn function, which is a subscription function for account statuses,
// and an array of testType structs representing the test cases.
// It iterates over each test case and sets up the necessary context and cancellation for the subscription.
// For each test case, it simulates backfill blocks and verifies the expected account events for each block.
// It also ensures that the subscription shuts down gracefully after completing the test cases.
func (s *BackendAccountStatusesSuite) subscribeToAccountStatuses(
	subscribeFn func(ctx context.Context, startValue interface{}, filter state_stream.AccountStatusFilter) subscription.Subscription,
	tests []testType,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Iterate over each test case
	for _, test := range tests {
		s.Run(test.name, func() {
			s.T().Logf("len(s.execDataMap) %d", len(s.execDataMap))

			// Add "backfill" block - blocks that are already in the database before the test starts
			// This simulates a subscription on a past block
			for i := 0; i <= test.highestBackfill; i++ {
				s.T().Logf("backfilling block %d", i)
				s.executionDataTracker.On("GetHighestHeight").
					Return(s.blocks[i].Header.Height)
			}

			// Set up subscription context and cancellation
			subCtx, subCancel := context.WithCancel(ctx)

			sub := subscribeFn(subCtx, test.startValue, test.filters)

			expectedMsgIndex := uint64(0)

			// Loop over all the blocks
			for i, b := range s.blocks {
				s.T().Logf("checking block %d %v", i, b.ID())

				// Simulate new exec data received.
				// Exec data for all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.executionDataTracker.On("GetHighestHeight").Unset()
					s.executionDataTracker.On("GetHighestHeight").
						Return(b.Header.Height)
					s.broadcaster.Publish()
				}

				expectedEvents := map[string]flow.EventsList{}
				for _, event := range s.blockEvents[b.ID()] {
					if test.filters.Match(event) {
						expectedEvents[accountAddress] = append(expectedEvents[accountAddress], event)
					}
				}

				// Consume execution data from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v: err: %v", b.Header.Height, b.ID(), sub.Err())

					resp, ok := v.(*AccountStatusesResponse)
					require.True(s.T(), ok, "unexpected response type: %T", v)

					assert.Equal(s.T(), b.Header.ID(), resp.BlockID)
					assert.Equal(s.T(), b.Header.Height, resp.Height)
					assert.Equal(s.T(), expectedMsgIndex, resp.MessageIndex)
					assert.Equal(s.T(), expectedEvents, resp.AccountEvents)
				}, 60*time.Second, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Header.Height, b.ID()))

				expectedMsgIndex++
			}

			// Make sure there are no new messages waiting. The channel should be opened with nothing waiting
			unittest.RequireNeverReturnBefore(s.T(), func() {
				<-sub.Channel()
			}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")

			// Stop the subscription
			subCancel()

			// Ensure subscription shuts down gracefully
			unittest.RequireReturnsBefore(s.T(), func() {
				v, ok := <-sub.Channel()
				assert.Nil(s.T(), v)
				assert.False(s.T(), ok)
				assert.ErrorIs(s.T(), sub.Err(), context.Canceled)
			}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")
		})
	}
}

// TestSubscribeAccountStatusesFromStartBlockID tests the SubscribeAccountStatusesFromStartBlockID method.
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatusesFromStartBlockID() {
	s.executionDataTracker.On(
		"GetStartHeightFromBlockID",
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	call := func(ctx context.Context, startValue interface{}, filter state_stream.AccountStatusFilter) subscription.Subscription {
		return s.backend.SubscribeAccountStatusesFromStartBlockID(ctx, startValue.(flow.Identifier), filter)
	}

	s.subscribeToAccountStatuses(call, s.subscribeFromStartBlockIdTestCases())
}

// TestSubscribeAccountStatusesFromStartHeight tests the SubscribeAccountStatusesFromStartHeight method.
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatusesFromStartHeight() {
	s.executionDataTracker.On(
		"GetStartHeightFromHeight",
		mock.AnythingOfType("uint64"),
	).Return(func(startHeight uint64) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
	}, nil)

	call := func(ctx context.Context, startValue interface{}, filter state_stream.AccountStatusFilter) subscription.Subscription {
		return s.backend.SubscribeAccountStatusesFromStartHeight(ctx, startValue.(uint64), filter)
	}

	s.subscribeToAccountStatuses(call, s.subscribeFromStartHeightTestCases())
}

// TestSubscribeAccountStatusesFromLatestBlock tests the SubscribeAccountStatusesFromLatestBlock method.
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatusesFromLatestBlock() {
	s.executionDataTracker.On(
		"GetStartHeightFromLatest",
		mock.Anything,
	).Return(func(ctx context.Context) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromLatest(ctx)
	}, nil)

	call := func(ctx context.Context, startValue interface{}, filter state_stream.AccountStatusFilter) subscription.Subscription {
		return s.backend.SubscribeAccountStatusesFromLatestBlock(ctx, filter)
	}

	s.subscribeToAccountStatuses(call, s.subscribeFromLatestTestCases())
}

// TestSubscribeAccountStatusesHandlesErrors tests handling o f expected errors in the SubscribeAccountStatuses.
func (s *BackendExecutionDataSuite) TestSubscribeAccountStatusesHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// mock block tracker for SubscribeBlocksFromStartBlockID
	s.executionDataTracker.On(
		"GetStartHeightFromBlockID",
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	s.Run("returns error for unindexed start blockID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatusesFromStartBlockID(subCtx, unittest.IdentifierFixture(), state_stream.AccountStatusFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.executionDataTracker.On(
		"GetStartHeightFromHeight",
		mock.AnythingOfType("uint64"),
	).Return(func(startHeight uint64) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
	}, nil)

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatusesFromStartHeight(subCtx, s.rootBlock.Header.Height-1, state_stream.AccountStatusFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	// make sure we're starting with a fresh cache
	s.execDataHeroCache.Clear()

	s.Run("returns error for unindexed start height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatusesFromStartHeight(subCtx, s.blocks[len(s.blocks)-1].Header.Height+10, state_stream.AccountStatusFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})
}

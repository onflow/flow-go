package backend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
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
	rootBlock := unittest.BlockFixture()
	parent := rootBlock.Header
	s.blockMap[rootBlock.Header.Height] = &rootBlock

	s.T().Logf("Generating %d blocks, root block: %d %s", blockCount, rootBlock.Header.Height, rootBlock.ID())

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
		rootBlock.Header.Height,
		rootBlock.Header.Height, // initialize with no downloaded data
		s.registersAsync,
		s.eventsIndex,
		false,
	)
	require.NoError(s.T(), err)
}

// TestSubscribeAccountStatuses tests the SubscribeAccountStatuses method.
// This test ensures that the SubscribeAccountStatuses method correctly subscribes to account status updates.
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatuses() {
	// Set up test context and cancellation function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	// Define the test type struct
	type testType struct {
		name            string                   // Test case name
		highestBackfill int                      // Highest backfill index
		startBlockID    flow.Identifier          // Start block ID
		startHeight     uint64                   // Start height
		filters         state_stream.EventFilter // Event filters
	}

	// Define base test cases
	baseTests := []testType{
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
			startHeight:     s.backend.rootBlockHeight, // start from root block
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1,     // backfill all blocks
			startBlockID:    s.backend.rootBlockID, // start from root block
			startHeight:     0,
		},
	}

	// Create variations for each of the base tests
	tests := make([]testType, 0, len(baseTests)*3)
	for _, test := range baseTests {
		addressFilter := state_stream.FieldFilter{
			FieldName:   "address",
			TargetValue: accountAddress,
		}
		t1 := test
		t1.name = fmt.Sprintf("%s - all events", test.name)
		t1.filters, err = state_stream.NewEventFilter(
			state_stream.DefaultEventFilterConfig,
			chainID.Chain(),
			[]string{string(testProtocolEventTypes[0]), string(testProtocolEventTypes[1]), string(testProtocolEventTypes[2])},
			nil,
			nil,
		)
		require.NoError(s.T(), err)
		for _, eventType := range testProtocolEventTypes {
			t1.filters.EventFieldFilters[eventType] = []state_stream.FieldFilter{addressFilter}
		}
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - some events", test.name)
		t2.filters, err = state_stream.NewEventFilter(
			state_stream.DefaultEventFilterConfig,
			chainID.Chain(),
			[]string{string(testProtocolEventTypes[0])},
			nil,
			nil,
		)
		require.NoError(s.T(), err)
		t2.filters.EventFieldFilters[testProtocolEventTypes[0]] = []state_stream.FieldFilter{addressFilter}
		tests = append(tests, t2)

		t3 := test
		t3.name = fmt.Sprintf("%s - no events", test.name)
		t3.filters, err = state_stream.NewEventFilter(
			state_stream.DefaultEventFilterConfig,
			chainID.Chain(),
			[]string{"flow.AccountKeyAdded"},
			nil,
			nil,
		)
		require.NoError(s.T(), err)
		t3.filters.EventFieldFilters["flow.AccountKeyAdded"] = []state_stream.FieldFilter{addressFilter}
		tests = append(tests, t3)
	}

	// Iterate over each test case
	for _, test := range tests {
		s.Run(test.name, func() {
			s.T().Logf("len(s.execDataMap) %d", len(s.execDataMap))

			// Add "backfill" block - blocks that are already in the database before the test starts
			// This simulates a subscription on a past block
			for i := 0; i <= test.highestBackfill; i++ {
				s.T().Logf("backfilling block %d", i)
				s.backend.setHighestHeight(s.blocks[i].Header.Height)
			}

			// Set up subscription context and cancellation
			subCtx, subCancel := context.WithCancel(ctx)
			sub := s.backend.SubscribeAccountStatuses(subCtx, test.startBlockID, test.startHeight, test.filters)

			expectedMsgIndex := uint64(0)

			// Loop over all of the blocks
			for i, b := range s.blocks {
				s.T().Logf("checking block %d %v", i, b.ID())

				// Simulate new exec data received.
				// Exec data for all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.backend.setHighestHeight(b.Header.Height)
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

// TestSubscribeAccountStatusesHandlesErrors tests handling o f expected errors in the SubscribeAccountStatuses.
func (s *BackendExecutionDataSuite) TestSubscribeAccountStatusesHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Run("returns error if both start blockID and start height are provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatuses(subCtx, unittest.IdentifierFixture(), 1, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()))
	})

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatuses(subCtx, flow.ZeroID, s.backend.rootBlockHeight-1, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for unindexed start blockID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatuses(subCtx, unittest.IdentifierFixture(), 0, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	// make sure we're starting with a fresh cache
	s.execDataHeroCache.Clear()

	s.Run("returns error for unindexed start height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatuses(subCtx, flow.ZeroID, s.blocks[len(s.blocks)-1].Header.Height+10, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})
}

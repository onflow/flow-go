package backend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
)

var testProtocolEventTypes = []flow.EventType{
	state_stream.CoreEventAccountCreated,
	state_stream.CoreEventAccountContractAdded,
	state_stream.CoreEventAccountContractUpdated,
}

// Define the test type struct
// The struct is used for testing different test cases of each endpoint from AccountStatusesBackend.
type testType struct {
	name            string // Test case name
	highestBackfill int    // Highest backfill index
	startValue      interface{}
	filters         state_stream.AccountStatusFilter // Event filters
}

// BackendAccountStatusesSuite is a test suite for the AccountStatusesBackend functionality.
// It is used to test the endpoints which enables users to subscribe to the streaming of account status changes.
// It verified that each of endpoints works properly with expected data being returned. Also the suite tests
// handling of expected errors in the SubscribeAccountStatuses.
type BackendAccountStatusesSuite struct {
	BackendExecutionDataSuite
	accountCreatedAddress  flow.Address
	accountContractAdded   flow.Address
	accountContractUpdated flow.Address
}

func TestBackendAccountStatusesSuite(t *testing.T) {
	suite.Run(t, new(BackendAccountStatusesSuite))
}

// generateProtocolMockEvents generates a set of mock events.
func (s *BackendAccountStatusesSuite) generateProtocolMockEvents() flow.EventsList {
	events := make([]flow.Event, 4)
	events = append(events, unittest.EventFixture(testEventTypes[0], 0, 0, unittest.IdentifierFixture(), 0))

	accountCreateEvent := generator.GenerateAccountCreateEvent(s.T(), s.accountCreatedAddress)
	accountCreateEvent.TransactionIndex = 1
	events = append(events, accountCreateEvent)

	accountContractAdded := generator.GenerateAccountContractEvent(s.T(), "AccountContractAdded", s.accountContractAdded)
	accountContractAdded.TransactionIndex = 2
	events = append(events, accountContractAdded)

	accountContractUpdated := generator.GenerateAccountContractEvent(s.T(), "AccountContractUpdated", s.accountContractUpdated)
	accountContractUpdated.TransactionIndex = 3
	events = append(events, accountContractUpdated)

	return events
}

// SetupTest initializes the test suite.
func (s *BackendAccountStatusesSuite) SetupTest() {
	blockCount := 5
	var err error
	s.SetupTestSuite(blockCount)

	addressGenerator := chainID.Chain().NewAddressGenerator()
	s.accountCreatedAddress, err = addressGenerator.NextAddress()
	require.NoError(s.T(), err)
	s.accountContractAdded, err = addressGenerator.NextAddress()
	require.NoError(s.T(), err)
	s.accountContractUpdated, err = addressGenerator.NextAddress()
	require.NoError(s.T(), err)

	parent := s.rootBlock.Header
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

	s.SetupTestMocks()
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
			[]string{s.accountCreatedAddress.HexWithPrefix(), s.accountContractAdded.HexWithPrefix(), s.accountContractUpdated.HexWithPrefix()},
		)
		require.NoError(s.T(), err)
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - some events", test.name)
		t2.filters, err = state_stream.NewAccountStatusFilter(
			state_stream.DefaultEventFilterConfig,
			chainID.Chain(),
			[]string{string(testProtocolEventTypes[0])},
			[]string{s.accountCreatedAddress.HexWithPrefix(), s.accountContractAdded.HexWithPrefix(), s.accountContractUpdated.HexWithPrefix()},
		)
		require.NoError(s.T(), err)
		tests = append(tests, t2)

		t3 := test
		t3.name = fmt.Sprintf("%s - no events", test.name)
		t3.filters, err = state_stream.NewAccountStatusFilter(
			state_stream.DefaultEventFilterConfig,
			chainID.Chain(),
			[]string{"flow.AccountKeyAdded"},
			[]string{s.accountCreatedAddress.HexWithPrefix(), s.accountContractAdded.HexWithPrefix(), s.accountContractUpdated.HexWithPrefix()},
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
						var address string
						switch event.Type {
						case state_stream.CoreEventAccountCreated:
							address = s.accountCreatedAddress.HexWithPrefix()
						case state_stream.CoreEventAccountContractAdded:
							address = s.accountContractAdded.HexWithPrefix()
						case state_stream.CoreEventAccountContractUpdated:
							address = s.accountContractUpdated.HexWithPrefix()
						}
						expectedEvents[address] = append(expectedEvents[address], event)
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

// TestSubscribeAccountStatusesHandlesErrors tests handling of expected errors in the SubscribeAccountStatuses.
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

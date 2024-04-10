package backend

import (
	"bytes"
	"context"
	"fmt"
	"sort"
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
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// eventsTestType represents a test scenario for subscribe events endpoints.
// The old version of test case is used to test SubscribeEvents as well.
// After removing SubscribeEvents endpoint testType struct as a test case can be used.
type eventsTestType struct {
	name            string
	highestBackfill int
	startBlockID    flow.Identifier
	startHeight     uint64
	filter          state_stream.EventFilter
}

// BackendEventsSuite is a test suite for the EventsBackend functionality.
// It is used to test the endpoints which enable users to subscribe to block events.
// It verifies that each endpoint works properly with the expected data being returned and tests
// handling of expected errors.
//
// Test cases cover various subscription methods:
// - Subscribing from a start block ID or start height (SubscribeEvents)
// - Subscribing from a start block ID (SubscribeEventsFromStartBlockID)
// - Subscribing from a start height (SubscribeEventsFromStartHeight)
// - Subscribing from the latest data (SubscribeEventsFromLatest)
//
// Each test case covers various scenarios and edge cases, thoroughly assessing the
// EventsBackend's subscription functionality and its ability to handle different
// starting points, event sources, and filtering criteria.
//
// The suite focuses on events extracted from local storage and extracted from ExecutionData,
// ensuring proper testing of event retrieval from both sources.
type BackendEventsSuite struct {
	BackendExecutionDataSuite
}

func TestBackendEventsSuite(t *testing.T) {
	suite.Run(t, new(BackendEventsSuite))
}

// SetupTest initializes the test suite.
func (s *BackendEventsSuite) SetupTest() {
	s.BackendExecutionDataSuite.SetupTest()
}

// setupFilterForTestCases sets up variations of test scenarios with different event filters
//
// This function takes an array of base testType structs and creates variations for each of them.
// For each base test case, it generates three variations:
// - All events: Includes all event types.
// - Some events: Includes only event types that match the provided filter.
// - No events: Includes a custom event type "A.0x1.NonExistent.Event".
func (s *BackendEventsSuite) setupFilterForTestCases(baseTests []eventsTestType) []eventsTestType {
	// create variations for each of the base test
	tests := make([]eventsTestType, 0, len(baseTests)*3)
	var err error

	for _, test := range baseTests {
		t1 := test
		t1.name = fmt.Sprintf("%s - all events", test.name)
		t1.filter = state_stream.EventFilter{}
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - some events", test.name)
		t2.filter, err = state_stream.NewEventFilter(state_stream.DefaultEventFilterConfig, chainID.Chain(), []string{string(testEventTypes[0])}, nil, nil)
		require.NoError(s.T(), err)
		tests = append(tests, t2)

		t3 := test
		t3.name = fmt.Sprintf("%s - no events", test.name)
		t3.filter, err = state_stream.NewEventFilter(state_stream.DefaultEventFilterConfig, chainID.Chain(), []string{"A.0x1.NonExistent.Event"}, nil, nil)
		require.NoError(s.T(), err)
		tests = append(tests, t3)
	}

	return tests
}

// setupLocalStorage prepares local storage for testing
func (s *BackendEventsSuite) setupLocalStorage() {
	s.SetupBackend(true)

	// events returned from the db are sorted by txID, txIndex, then eventIndex.
	// reproduce that here to ensure output order works as expected
	blockEvents := make(map[flow.Identifier][]flow.Event)
	for _, b := range s.blocks {
		events := make([]flow.Event, len(s.blockEvents[b.ID()]))
		for i, event := range s.blockEvents[b.ID()] {
			events[i] = event
		}
		sort.Slice(events, func(i, j int) bool {
			cmp := bytes.Compare(events[i].TransactionID[:], events[j].TransactionID[:])
			if cmp == 0 {
				if events[i].TransactionIndex == events[j].TransactionIndex {
					return events[i].EventIndex < events[j].EventIndex
				}
				return events[i].TransactionIndex < events[j].TransactionIndex
			}
			return cmp < 0
		})
		blockEvents[b.ID()] = events
	}

	s.events.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		mocks.StorageMapGetter(blockEvents),
	)

	reporter := syncmock.NewIndexReporter(s.T())
	reporter.On("LowestIndexedHeight").Return(s.blocks[0].Header.Height, nil)
	reporter.On("HighestIndexedHeight").Return(s.blocks[len(s.blocks)-1].Header.Height, nil)
	err := s.eventsIndex.Initialize(reporter)
	s.Require().NoError(err)
}

// TestSubscribeEventsFromExecutionData tests the SubscribeEvents method happy path for events
// extracted from ExecutionData
func (s *BackendEventsSuite) TestSubscribeEventsFromExecutionData() {
	s.runTestSubscribeEvents()
}

// TestSubscribeEventsFromLocalStorage tests the SubscribeEvents method happy path for events
// extracted from local storage
func (s *BackendEventsSuite) TestSubscribeEventsFromLocalStorage() {
	s.setupLocalStorage()
	s.runTestSubscribeEvents()
}

// TestSubscribeEventsFromStartBlockIDFromExecutionData tests the SubscribeEventsFromStartBlockID method happy path for events
// extracted from ExecutionData
func (s *BackendEventsSuite) TestSubscribeEventsFromStartBlockIDFromExecutionData() {
	s.runTestSubscribeEventsFromStartBlockID()
}

// TestSubscribeEventsFromStartBlockIDFromLocalStorage tests the SubscribeEventsFromStartBlockID method happy path for events
// extracted from local storage
func (s *BackendEventsSuite) TestSubscribeEventsFromStartBlockIDFromLocalStorage() {
	s.setupLocalStorage()
	s.runTestSubscribeEventsFromStartBlockID()
}

// TestSubscribeEventsFromStartHeightFromExecutionData tests the SubscribeEventsFromStartHeight method happy path for events
// extracted from ExecutionData
func (s *BackendEventsSuite) TestSubscribeEventsFromStartHeightFromExecutionData() {
	s.runTestSubscribeEventsFromStartHeight()
}

// TestSubscribeEventsFromStartHeightFromLocalStorage tests the SubscribeEventsFromStartHeight method happy path for events
// extracted from local storage
func (s *BackendEventsSuite) TestSubscribeEventsFromStartHeightFromLocalStorage() {
	s.setupLocalStorage()
	s.runTestSubscribeEventsFromStartHeight()
}

// TestSubscribeEventsFromLatestFromExecutionData tests the SubscribeEventsFromLatest method happy path for events
// extracted from ExecutionData
func (s *BackendEventsSuite) TestSubscribeEventsFromLatestFromExecutionData() {
	s.runTestSubscribeEventsFromLatest()
}

// TestSubscribeEventsFromLatestFromLocalStorage tests the SubscribeEventsFromLatest method happy path for events
// extracted from local storage
func (s *BackendEventsSuite) TestSubscribeEventsFromLatestFromLocalStorage() {
	s.setupLocalStorage()
	s.runTestSubscribeEventsFromLatest()
}

// runTestSubscribeEvents runs the test suite for SubscribeEvents subscription
func (s *BackendEventsSuite) runTestSubscribeEvents() {
	tests := []eventsTestType{
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
			startHeight:     s.rootBlock.Header.Height, // start from root block
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1,       // backfill all blocks
			startBlockID:    s.rootBlock.Header.ID(), // start from root block
			startHeight:     0,
		},
	}

	call := func(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
		return s.backend.SubscribeEvents(ctx, startBlockID, startHeight, filter)
	}

	s.subscribe(call, s.requireEventsResponse, s.setupFilterForTestCases(tests))
}

// runTestSubscribeEventsFromStartBlockID runs the test suite for SubscribeEventsFromStartBlockID subscription
func (s *BackendEventsSuite) runTestSubscribeEventsFromStartBlockID() {
	tests := []eventsTestType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startBlockID:    s.rootBlock.ID(),
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
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startBlockID:    s.rootBlock.ID(),  // start from root block
		},
	}

	s.executionDataTracker.On(
		"GetStartHeightFromBlockID",
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	call := func(ctx context.Context, startBlockID flow.Identifier, _ uint64, filter state_stream.EventFilter) subscription.Subscription {
		return s.backend.SubscribeEventsFromStartBlockID(ctx, startBlockID, filter)
	}

	s.subscribe(call, s.requireEventsResponse, s.setupFilterForTestCases(tests))
}

// runTestSubscribeEventsFromStartHeight runs the test suite for SubscribeEventsFromStartHeight subscription
func (s *BackendEventsSuite) runTestSubscribeEventsFromStartHeight() {
	tests := []eventsTestType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startHeight:     s.rootBlock.Header.Height,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startHeight:     s.blocks[0].Header.Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startHeight:     s.blocks[0].Header.Height,
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1,         // backfill all blocks
			startHeight:     s.rootBlock.Header.Height, // start from root block
		},
	}

	s.executionDataTracker.On(
		"GetStartHeightFromHeight",
		mock.AnythingOfType("uint64"),
	).Return(func(startHeight uint64) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
	}, nil)

	call := func(ctx context.Context, _ flow.Identifier, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
		return s.backend.SubscribeEventsFromStartHeight(ctx, startHeight, filter)
	}

	s.subscribe(call, s.requireEventsResponse, s.setupFilterForTestCases(tests))
}

// runTestSubscribeEventsFromLatest runs the test suite for SubscribeEventsFromLatest subscription
func (s *BackendEventsSuite) runTestSubscribeEventsFromLatest() {
	tests := []eventsTestType{
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

	call := func(ctx context.Context, _ flow.Identifier, _ uint64, filter state_stream.EventFilter) subscription.Subscription {
		return s.backend.SubscribeEventsFromLatest(ctx, filter)
	}

	s.subscribe(call, s.requireEventsResponse, s.setupFilterForTestCases(tests))
}

// subscribe is a helper function to run test scenarios for event subscription in the BackendEventsSuite.
// It covers various scenarios for subscribing, handling backfill, and receiving block updates.
// The test cases include scenarios for different event filters.
//
// Parameters:
//
//   - subscribeFn: A function representing the subscription method to be tested.
//     It takes a context, startBlockID, startHeight, and filter as parameters
//     and returns a subscription.Subscription.
//
//   - requireFn: A function responsible for validating that the received information
//     matches the expected data. It takes an actual interface{} and an expected *EventsResponse as parameters.
//
//   - tests: A slice of testType representing different test scenarios for subscriptions.
//
// The function performs the following steps for each test case:
//
//  1. Initializes the test context and cancellation function.
//  2. Iterates through the provided test cases.
//  3. For each test case, sets up a executionDataTracker mock if there are blocks to backfill.
//  4. Mocks the latest sealed block if no startBlockID or startHeight is provided.
//  5. Subscribes using the provided subscription function.
//  6. Simulates the reception of new blocks and consumes them from the subscription channel.
//  7. Ensures that there are no new messages waiting after all blocks have been processed.
//  8. Cancels the subscription and ensures it shuts down gracefully.
func (s *BackendEventsSuite) subscribe(
	subscribeFn func(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription,
	requireFn func(interface{}, *EventsResponse),
	tests []eventsTestType,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, test := range tests {
		s.Run(test.name, func() {
			// add "backfill" block - blocks that are already in the database before the test starts
			// this simulates a subscription on a past block
			if test.highestBackfill > 0 {
				s.highestBlockHeader = s.blocks[test.highestBackfill].Header
			}

			subCtx, subCancel := context.WithCancel(ctx)

			// mock latest sealed if test case no start value provided
			if test.startBlockID == flow.ZeroID && test.startHeight == 0 {
				s.snapshot.On("Head").Unset()
				s.snapshot.On("Head").Return(s.rootBlock.Header, nil).Once()
			}

			sub := subscribeFn(subCtx, test.startBlockID, test.startHeight, test.filter)

			// loop over all blocks
			for i, b := range s.blocks {
				s.T().Logf("checking block %d %v %d", i, b.ID(), b.Header.Height)

				// simulate new block received.
				// all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.highestBlockHeader = b.Header

					s.broadcaster.Publish()
				}

				var expectedEvents flow.EventsList
				for _, event := range s.blockEvents[b.ID()] {
					if test.filter.Match(event) {
						expectedEvents = append(expectedEvents, event)
					}
				}

				// consume events response from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %x %v: err: %v", b.Header.Height, b.ID(), sub.Err())

					expected := &EventsResponse{
						BlockID:        b.ID(),
						Height:         b.Header.Height,
						Events:         expectedEvents,
						BlockTimestamp: b.Header.Timestamp,
					}
					requireFn(v, expected)

				}, time.Second, fmt.Sprintf("timed out waiting for block %d %v", b.Header.Height, b.ID()))
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

// requireEventsResponse ensures that the received event information matches the expected data.
func (s *BackendEventsSuite) requireEventsResponse(v interface{}, expected *EventsResponse) {
	actual, ok := v.(*EventsResponse)
	require.True(s.T(), ok, "unexpected response type: %T", v)

	assert.Equal(s.T(), expected.BlockID, actual.BlockID)
	assert.Equal(s.T(), expected.Height, actual.Height)
	assert.Equal(s.T(), expected.Events, actual.Events)
	assert.Equal(s.T(), expected.BlockTimestamp, actual.BlockTimestamp)
}

// TestSubscribeEventsHandlesErrors tests error handling for SubscribeEvents subscription
//
// Test Cases:
//
// 1. Returns error if both start blockID and start height are provided:
//   - Ensures that providing both start blockID and start height results in an InvalidArgument error.
//
// 2. Returns error for start height before root height:
//   - Validates that attempting to subscribe with a start height before the root height results in an InvalidArgument error.
//
// 3. Returns error for unindexed start blockID:
//   - Tests that subscribing with an unindexed start blockID results in a NotFound error.
//
// 4. Returns error for unindexed start height:
//   - Tests that subscribing with an unindexed start height results in a NotFound error.
//
// 5. Returns error for uninitialized index:
//   - Ensures that subscribing with an uninitialized index results in a FailedPrecondition error.
//
// 6. Returns error for start below lowest indexed:
//   - Validates that subscribing with a start height below the lowest indexed height results in an InvalidArgument error.
//
// 7. Returns error for start above highest indexed:
//   - Validates that subscribing with a start height above the highest indexed height results in an InvalidArgument error.
func (s *BackendExecutionDataSuite) TestSubscribeEventsHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Run("returns error if both start blockID and start height are provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeEvents(subCtx, unittest.IdentifierFixture(), 1, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()))
	})

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeEvents(subCtx, flow.ZeroID, s.rootBlock.Header.Height-1, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for unindexed start blockID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeEvents(subCtx, unittest.IdentifierFixture(), 0, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	// make sure we're starting with a fresh cache
	s.execDataHeroCache.Clear()

	s.Run("returns error for unindexed start height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeEvents(subCtx, flow.ZeroID, s.blocks[len(s.blocks)-1].Header.Height+10, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	// Unset GetStartHeight to mock new behavior instead of default one
	s.executionDataTracker.On("GetStartHeight", mock.Anything, mock.Anything).Unset()

	s.Run("returns error for uninitialized index", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeight", subCtx, flow.ZeroID, uint64(0)).
			Return(uint64(0), status.Errorf(codes.FailedPrecondition, "failed to get lowest indexed height: %v", indexer.ErrIndexNotInitialized)).
			Once()

		// Note: eventIndex.Initialize() is not called in this test
		sub := s.backend.SubscribeEvents(subCtx, flow.ZeroID, 0, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.FailedPrecondition, status.Code(sub.Err()), "expected FailedPrecondition, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for start below lowest indexed", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeight", subCtx, flow.ZeroID, s.blocks[0].Header.Height).
			Return(uint64(0), status.Errorf(codes.InvalidArgument, "start height %d is lower than lowest indexed height %d", s.blocks[0].Header.Height, 0)).
			Once()

		sub := s.backend.SubscribeEvents(subCtx, flow.ZeroID, s.blocks[0].Header.Height, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for start above highest indexed", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeight", subCtx, flow.ZeroID, s.blocks[len(s.blocks)-1].Header.Height).
			Return(uint64(0), status.Errorf(codes.InvalidArgument, "start height %d is higher than highest indexed height %d", s.blocks[len(s.blocks)-1].Header.Height, s.blocks[0].Header.Height)).
			Once()

		sub := s.backend.SubscribeEvents(subCtx, flow.ZeroID, s.blocks[len(s.blocks)-1].Header.Height, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})
}

// TestSubscribeEventsFromStartBlockIDHandlesErrors tests error handling for SubscribeEventsFromStartBlockID subscription
//
// Test Cases:
//
// 1. Returns error for unindexed start blockID:
//   - Ensures that subscribing with an unindexed start blockID results in a NotFound error.
//
// 2. Returns error for uninitialized index:
//   - Ensures that subscribing with an uninitialized index results in a FailedPrecondition error.
//
// 3. Returns error for start below lowest indexed:
//   - Validates that subscribing with a start blockID below the lowest indexed height results in an InvalidArgument error.
//
// 4. Returns error for start above highest indexed:
//   - Validates that subscribing with a start blockID above the highest indexed height results in an InvalidArgument error.
func (s *BackendExecutionDataSuite) TestSubscribeEventsFromStartBlockIDHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.executionDataTracker.On(
		"GetStartHeightFromBlockID",
		mock.Anything,
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	s.Run("returns error for unindexed start blockID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeEventsFromStartBlockID(subCtx, unittest.IdentifierFixture(), state_stream.EventFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	// Unset GetStartHeightFromBlockID to mock new behavior instead of default one
	s.executionDataTracker.On("GetStartHeightFromBlockID", mock.Anything).Unset()

	s.Run("returns error for uninitialized index", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromBlockID", flow.ZeroID).
			Return(uint64(0), status.Errorf(codes.FailedPrecondition, "failed to get lowest indexed height: %v", indexer.ErrIndexNotInitialized)).
			Once()

		// Note: eventIndex.Initialize() is not called in this test
		sub := s.backend.SubscribeEventsFromStartBlockID(subCtx, flow.ZeroID, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.FailedPrecondition, status.Code(sub.Err()), "expected FailedPrecondition, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for start below lowest indexed", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromBlockID", s.blocks[0].ID()).
			Return(uint64(0), status.Errorf(codes.InvalidArgument, "start height %d is lower than lowest indexed height %d", s.blocks[0].Header.Height, 0)).
			Once()

		sub := s.backend.SubscribeEventsFromStartBlockID(subCtx, s.blocks[0].ID(), state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for start above highest indexed", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromBlockID", s.blocks[len(s.blocks)-1].ID()).
			Return(uint64(0), status.Errorf(codes.InvalidArgument, "start height %d is higher than highest indexed height %d", s.blocks[len(s.blocks)-1].Header.Height, s.blocks[0].Header.Height)).
			Once()

		sub := s.backend.SubscribeEventsFromStartBlockID(subCtx, s.blocks[len(s.blocks)-1].ID(), state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})
}

// TestSubscribeEventsFromStartHeightHandlesErrors tests error handling for SubscribeEventsFromStartHeight subscription.
//
// Test Cases:
//
// 1. Returns error for start height before root height:
//   - Validates that attempting to subscribe with a start height before the root height results in an InvalidArgument error.
//
// 2. Returns error for unindexed start height:
//   - Tests that subscribing with an unindexed start height results in a NotFound error.
//
// 3. Returns error for uninitialized index:
//   - Ensures that subscribing with an uninitialized index results in a FailedPrecondition error.
//
// 4. Returns error for start below lowest indexed:
//   - Validates that subscribing with a start height below the lowest indexed height results in an InvalidArgument error.
//
// 5. Returns error for start above highest indexed:
//   - Validates that subscribing with a start height above the highest indexed height results in an InvalidArgument error.
func (s *BackendExecutionDataSuite) TestSubscribeEventsFromStartHeightHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.executionDataTracker.On(
		"GetStartHeightFromHeight",
		mock.Anything,
	).Return(func(startHeight uint64) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
	}, nil)

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeEventsFromStartHeight(subCtx, s.rootBlock.Header.Height-1, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	// make sure we're starting with a fresh cache
	s.execDataHeroCache.Clear()

	s.Run("returns error for unindexed start height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeEventsFromStartHeight(subCtx, s.blocks[len(s.blocks)-1].Header.Height+10, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	// Unset GetStartHeightFromHeight to mock new behavior instead of default one
	s.executionDataTracker.On("GetStartHeightFromHeight", mock.Anything).Unset()

	s.Run("returns error for uninitialized index", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromHeight", s.blocks[0].Header.Height).
			Return(uint64(0), status.Errorf(codes.FailedPrecondition, "failed to get lowest indexed height: %v", indexer.ErrIndexNotInitialized)).
			Once()

		// Note: eventIndex.Initialize() is not called in this test
		sub := s.backend.SubscribeEventsFromStartHeight(subCtx, s.blocks[0].Header.Height, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.FailedPrecondition, status.Code(sub.Err()), "expected FailedPrecondition, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for start below lowest indexed", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromHeight", s.blocks[0].Header.Height).
			Return(uint64(0), status.Errorf(codes.InvalidArgument, "start height %d is lower than lowest indexed height %d", s.blocks[0].Header.Height, 0)).
			Once()

		sub := s.backend.SubscribeEventsFromStartHeight(subCtx, s.blocks[0].Header.Height, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for start above highest indexed", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		s.executionDataTracker.On("GetStartHeightFromHeight", s.blocks[len(s.blocks)-1].Header.Height).
			Return(uint64(0), status.Errorf(codes.InvalidArgument, "start height %d is higher than highest indexed height %d", s.blocks[len(s.blocks)-1].Header.Height, s.blocks[0].Header.Height)).
			Once()

		sub := s.backend.SubscribeEventsFromStartHeight(subCtx, s.blocks[len(s.blocks)-1].Header.Height, state_stream.EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})
}

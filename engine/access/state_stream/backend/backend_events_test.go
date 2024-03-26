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
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// testType represents a test scenario for subscribing
type testType struct {
	name            string
	highestBackfill int
	startBlockID    flow.Identifier
	startHeight     uint64
	filter          state_stream.EventFilter
}

type BackendEventsSuite struct {
	BackendExecutionDataSuite
}

func TestBackendEventsSuite(t *testing.T) {
	suite.Run(t, new(BackendEventsSuite))
}

func (s *BackendEventsSuite) SetupTest() {
	s.BackendExecutionDataSuite.SetupTest()
}

func (s *BackendEventsSuite) subscribeFromStartBlockIdTestCases() []testType {
	baseTests := []testType{
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

	return s.setupFilterForTestCases(baseTests)
}

func (s *BackendEventsSuite) subscribeFromStartHeightTestCases() []testType {
	baseTests := []testType{
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

	return s.setupFilterForTestCases(baseTests)
}

// subscribeFromLatestTestCases generates variations of testType scenarios for subscriptions
// starting from the latest sealed block. It is designed to test the subscription functionality when the subscription
// starts from the latest available block, either sealed or finalized.
func (s *BackendEventsSuite) subscribeFromLatestTestCases() []testType {
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

	return s.setupFilterForTestCases(baseTests)
}

func (s *BackendEventsSuite) setupFilterForTestCases(baseTests []testType) []testType {
	// create variations for each of the base test
	tests := make([]testType, 0, len(baseTests)*3)
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

// TestSubscribeEventsFromExecutionData tests the SubscribeEvents method happy path for events
// extracted from ExecutionData
func (s *BackendEventsSuite) TestSubscribeEventsFromExecutionData() {
	s.runTestSubscribeEvents()
}

// TestSubscribeEventsFromLocalStorage tests the SubscribeEvents method happy path for events
// extracted from local storage
func (s *BackendEventsSuite) TestSubscribeEventsFromLocalStorage() {
	s.backend.useIndex = true

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

	s.runTestSubscribeEvents()
}

func (s *BackendEventsSuite) runTestSubscribeEvents() {
	tests := []testType{
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

// TestSubscribeEventsFromStartBlockID tests the SubscribeEventsFromStartBlockID method.
func (s *BackendEventsSuite) TestSubscribeEventsFromStartBlockID() {
	s.executionDataTracker.On(
		"GetStartHeightFromBlockID",
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	call := func(ctx context.Context, startBlockID flow.Identifier, _ uint64, filter state_stream.EventFilter) subscription.Subscription {
		return s.backend.SubscribeEventsFromStartBlockID(ctx, startBlockID, filter)
	}

	s.subscribe(call, s.requireEventsResponse, s.subscribeFromStartBlockIdTestCases())
}

// TestSubscribeEventsFromStartHeight tests the SubscribeEventsFromStartHeight method.
func (s *BackendEventsSuite) TestSubscribeEventsFromStartHeight() {
	s.executionDataTracker.On(
		"GetStartHeightFromHeight",
		mock.AnythingOfType("uint64"),
	).Return(func(startHeight uint64) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
	}, nil)

	call := func(ctx context.Context, _ flow.Identifier, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
		return s.backend.SubscribeEventsFromStartHeight(ctx, startHeight, filter)
	}

	s.subscribe(call, s.requireEventsResponse, s.subscribeFromStartHeightTestCases())
}

// TestSubscribeEventsFromLatest tests the SubscribeEventsFromLatest method.
func (s *BackendEventsSuite) TestSubscribeEventsFromLatest() {
	s.executionDataTracker.On(
		"GetStartHeightFromLatest",
		mock.Anything,
	).Return(func(ctx context.Context) (uint64, error) {
		return s.executionDataTrackerReal.GetStartHeightFromLatest(ctx)
	}, nil)

	call := func(ctx context.Context, _ flow.Identifier, _ uint64, filter state_stream.EventFilter) subscription.Subscription {
		return s.backend.SubscribeEventsFromLatest(ctx, filter)
	}

	s.subscribe(call, s.requireEventsResponse, s.subscribeFromLatestTestCases())
}

func (s *BackendEventsSuite) subscribe(
	subscribeFn func(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription,
	requireFn func(interface{}, *EventsResponse),
	tests []testType,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, test := range tests {
		s.Run(test.name, func() {
			// add "backfill" block - blocks that are already in the database before the test starts
			// this simulates a subscription on a past block
			if test.highestBackfill > 0 {
				s.executionDataTracker.On("GetHighestHeight").
					Return(s.blocks[test.highestBackfill].Header.Height).Maybe()
			}

			subCtx, subCancel := context.WithCancel(ctx)
			expectedMsgIndexCounter := counters.NewMonotonousCounter(0)

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
					s.executionDataTracker.On("GetHighestHeight").Unset()
					s.executionDataTracker.On("GetHighestHeight").
						Return(b.Header.Height).Maybe()

					s.broadcaster.Publish()
				}

				var expectedEvents flow.EventsList
				for _, event := range s.blockEvents[b.ID()] {
					if test.filter.Match(event) {
						expectedEvents = append(expectedEvents, event)
					}
				}

				// consume block from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %x %v: err: %v", b.Header.Height, b.ID(), sub.Err())

					expected := &EventsResponse{
						BlockID:        b.ID(),
						Height:         b.Header.Height,
						Events:         expectedEvents,
						BlockTimestamp: b.Header.Timestamp,
						MessageIndex:   expectedMsgIndexCounter.Value(),
					}
					requireFn(v, expected)

					wasSet := expectedMsgIndexCounter.Set(expectedMsgIndexCounter.Value() + 1)
					require.True(s.T(), wasSet)

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
	//assert.Equal(s.T(), expected.BlockTimestamp, actual.BlockTimestamp)
	assert.Equal(s.T(), expected.MessageIndex, actual.MessageIndex)
}

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

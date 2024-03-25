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
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

type BackendEventsSuite struct {
	BackendExecutionDataSuite
}

func TestBackendEventsSuite(t *testing.T) {
	suite.Run(t, new(BackendEventsSuite))
}

func (s *BackendEventsSuite) SetupTest() {
	s.BackendExecutionDataSuite.SetupTest()
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	type testType struct {
		name            string
		highestBackfill int
		startBlockID    flow.Identifier
		startHeight     uint64
		filters         state_stream.EventFilter
	}

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
			startHeight:     s.rootBlock.Header.Height, // start from root block
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1,       // backfill all blocks
			startBlockID:    s.rootBlock.Header.ID(), // start from root block
			startHeight:     0,
		},
	}

	// create variations for each of the base test
	tests := make([]testType, 0, len(baseTests)*3)
	for _, test := range baseTests {
		t1 := test
		t1.name = fmt.Sprintf("%s - all events", test.name)
		t1.filters = state_stream.EventFilter{}
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - some events", test.name)
		t2.filters, err = state_stream.NewEventFilter(state_stream.DefaultEventFilterConfig, chainID.Chain(), []string{string(testEventTypes[0])}, nil, nil)
		require.NoError(s.T(), err)
		tests = append(tests, t2)

		t3 := test
		t3.name = fmt.Sprintf("%s - no events", test.name)
		t3.filters, err = state_stream.NewEventFilter(state_stream.DefaultEventFilterConfig, chainID.Chain(), []string{"A.0x1.NonExistent.Event"}, nil, nil)
		require.NoError(s.T(), err)
		tests = append(tests, t3)
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			s.T().Logf("len(s.execDataMap) %d", len(s.execDataMap))

			// add "backfill" block - blocks that are already in the database before the test starts
			// this simulates a subscription on a past block
			for i := 0; i <= test.highestBackfill; i++ {
				s.T().Logf("backfilling block %d", i)
				s.executionDataTracker.On("GetHighestHeight").
					Return(s.blocks[i].Header.Height).Maybe()
			}

			subCtx, subCancel := context.WithCancel(ctx)
			sub := s.backend.SubscribeEvents(subCtx, test.startBlockID, test.startHeight, test.filters)

			// loop over all of the blocks
			for i, b := range s.blocks {
				s.T().Logf("checking block %d %v", i, b.ID())

				// simulate new exec data received.
				// exec data for all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.T().Logf("checking block %d %v", i, b.ID())

					s.executionDataTracker.On("GetHighestHeight").Unset()
					s.executionDataTracker.On("GetHighestHeight").
						Return(b.Header.Height).Maybe()

					s.broadcaster.Publish()
				}

				var expectedEvents flow.EventsList
				for _, event := range s.blockEvents[b.ID()] {
					if test.filters.Match(event) {
						expectedEvents = append(expectedEvents, event)
					}
				}

				// consume execution data from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v: err: %v", b.Header.Height, b.ID(), sub.Err())

					resp, ok := v.(*EventsResponse)
					require.True(s.T(), ok, "unexpected response type: %T", v)

					assert.Equal(s.T(), b.Header.ID(), resp.BlockID)
					assert.Equal(s.T(), b.Header.Height, resp.Height)
					assert.Equal(s.T(), expectedEvents, resp.Events)
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

package state_stream

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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

// TestSubscribeEvents tests the SubscribeEvents method happy path
func (s *BackendEventsSuite) TestSubscribeEvents() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	type testType struct {
		name            string
		highestBackfill int
		startBlockID    flow.Identifier
		startHeight     uint64
		filters         EventFilter
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
			startHeight:     s.Blocks[0].Header.Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.Blocks) - 1, // backfill all blocks
			startBlockID:    s.Blocks[0].ID(),
			startHeight:     0,
		},
		{
			name:            "happy path - start from root block by height",
			highestBackfill: len(s.Blocks) - 1, // backfill all blocks
			startBlockID:    flow.ZeroID,
			startHeight:     s.Backend.rootBlockHeight, // start from root block
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.Blocks) - 1,     // backfill all blocks
			startBlockID:    s.Backend.rootBlockID, // start from root block
			startHeight:     0,
		},
	}

	// supports simple address comparisions for testing
	chain := flow.MonotonicEmulator.Chain()

	// create variations for each of the base test
	tests := make([]testType, 0, len(baseTests)*3)
	for _, test := range baseTests {
		t1 := test
		t1.name = fmt.Sprintf("%s - all events", test.name)
		t1.filters = EventFilter{}
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - some events", test.name)
		t2.filters, err = NewEventFilter(DefaultEventFilterConfig, chain, []string{string(TestEventTypes[0])}, nil, nil)
		require.NoError(s.T(), err)
		tests = append(tests, t2)

		t3 := test
		t3.name = fmt.Sprintf("%s - no events", test.name)
		t3.filters, err = NewEventFilter(DefaultEventFilterConfig, chain, []string{"A.0x1.NonExistent.Event"}, nil, nil)
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
				s.Backend.SetHighestHeight(s.Blocks[i].Header.Height)
			}

			subCtx, subCancel := context.WithCancel(ctx)
			sub := s.Backend.SubscribeEvents(subCtx, test.startBlockID, test.startHeight, test.filters)

			// loop over all of the blocks
			for i, b := range s.Blocks {
				s.T().Logf("checking block %d %v", i, b.ID())

				// simulate new exec data received.
				// exec data for all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.Backend.SetHighestHeight(b.Header.Height)
					s.broadcaster.Publish()
				}

				expectedEvents := flow.EventsList{}
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

		sub := s.Backend.SubscribeEvents(subCtx, unittest.IdentifierFixture(), 1, EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()))
	})

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.Backend.SubscribeEvents(subCtx, flow.ZeroID, s.Backend.rootBlockHeight-1, EventFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for unindexed start blockID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.Backend.SubscribeEvents(subCtx, unittest.IdentifierFixture(), 0, EventFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	// make sure we're starting with a fresh cache
	s.execDataHeroCache.Clear()

	s.Run("returns error for unindexed start height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.Backend.SubscribeEvents(subCtx, flow.ZeroID, s.Blocks[len(s.Blocks)-1].Header.Height+10, EventFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})
}

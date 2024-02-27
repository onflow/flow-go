package backend

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

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

var testCoreEventTypes = []flow.EventType{
	"flow.AccountCreated",
	"flow.AccountKeyAdded",
	"flow.AccountKeyRemoved",
}

// BackendAccountStatusesSuite is the test suite for AccountStatusesBackend.
type BackendAccountStatusesSuite struct {
	BackendExecutionDataSuite
}

func TestBackendAccountStatusesSuite(t *testing.T) {
	suite.Run(t, new(BackendAccountStatusesSuite))
}

func (s *BackendAccountStatusesSuite) SetupTest() {
	s.BackendExecutionDataSuite.SetupTest()
}

// TestSubscribeAccountStatuses tests the SubscribeAccountStatuses method happy path
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatuses() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	type testType struct {
		name            string
		highestBackfill int
		startBlockID    flow.Identifier
		startHeight     uint64
		filters         state_stream.StatusFilter
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
			startHeight:     s.backend.rootBlockHeight, // start from root block
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1,     // backfill all blocks
			startBlockID:    s.backend.rootBlockID, // start from root block
			startHeight:     0,
		},
	}

	// create variations for each of the base test
	tests := make([]testType, 0, len(baseTests)*3)
	for _, test := range baseTests {
		t1 := test
		t1.name = fmt.Sprintf("%s - all events", test.name)
		t1.filters = state_stream.StatusFilter{}
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - some events", test.name)
		t2.filters, err = state_stream.NewStatusFilter([]string{string(testEventTypes[0])}, chainID.Chain())
		require.NoError(s.T(), err)
		tests = append(tests, t2)

		t3 := test
		t3.name = fmt.Sprintf("%s - no events", test.name)
		t3.filters, err = state_stream.NewStatusFilter([]string{"A.0x1.NonExistent.Event"}, chainID.Chain())
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
				s.backend.setHighestHeight(s.blocks[i].Header.Height)
			}

			subCtx, subCancel := context.WithCancel(ctx)
			sub := s.backend.SubscribeAccountStatuses(subCtx, test.startBlockID, test.startHeight, test.filters)

			expectedMsgIndex := uint64(0)

			// loop over all of the blocks
			for i, b := range s.blocks {
				s.T().Logf("checking block %d %v", i, b.ID())

				// simulate new exec data received.
				// exec data for all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.backend.setHighestHeight(b.Header.Height)
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

					resp, ok := v.(*AccountStatusesResponse)
					require.True(s.T(), ok, "unexpected response type: %T", v)

					assert.Equal(s.T(), b.Header.ID(), resp.BlockID)
					assert.Equal(s.T(), expectedEvents, resp.Events)
					assert.Equal(s.T(), expectedMsgIndex, resp.MessageIndex)
				}, time.Second, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Header.Height, b.ID()))

				expectedMsgIndex++
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

// TestSubscribeAccountStatusesHandlesErrors tests handling of expected errors in the SubscribeAccountStatuses.
func (s *BackendExecutionDataSuite) TestSubscribeAccountStatusesHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Run("returns error if both start blockID and start height are provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatuses(subCtx, unittest.IdentifierFixture(), 1, state_stream.StatusFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()))
	})

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatuses(subCtx, flow.ZeroID, s.backend.rootBlockHeight-1, state_stream.StatusFilter{})
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for unindexed start blockID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatuses(subCtx, unittest.IdentifierFixture(), 0, state_stream.StatusFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	// make sure we're starting with a fresh cache
	s.execDataHeroCache.Clear()

	s.Run("returns error for unindexed start height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeAccountStatuses(subCtx, flow.ZeroID, s.blocks[len(s.blocks)-1].Header.Height+10, state_stream.StatusFilter{})
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})
}

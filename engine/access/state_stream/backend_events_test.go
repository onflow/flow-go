package state_stream

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
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

// test involves loading exec data and extracting events
// need N blocks with a set of M events
// Need to test:
// * no results
// * all results
// * partial results
// For each, thest using the same 3 cases as exec data streaming

func (s *BackendEventsSuite) TestSubscribeEvents() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
			startHeight:     s.blocks[0].Header.Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startBlockID:    s.blocks[0].ID(),
			startHeight:     0,
		},
	}

	tests := make([]testType, 0, len(baseTests)*3)
	for _, test := range baseTests {
		t1 := test
		t1.name = fmt.Sprintf("%s - all events", test.name)
		t1.filters = EventFilter{}
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - some events", test.name)
		t2.filters = NewEventFilter([]string{string(testEventTypes[0])}, nil, nil)
		tests = append(tests, t2)

		t3 := test
		t3.name = fmt.Sprintf("%s - no events", test.name)
		t3.filters = NewEventFilter([]string{"A.0x1.NonExistent.Event"}, nil, nil)
		tests = append(tests, t3)
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			s.T().Logf("len(s.execDataMap) %d", len(s.execDataMap))

			for i := 0; i <= test.highestBackfill; i++ {
				s.T().Logf("backfilling block %d", i)
				execData := s.execDataMap[s.blocks[i].ID()]
				s.execDataDistributor.OnExecutionDataReceived(execData)
			}

			subCtx, subCancel := context.WithCancel(ctx)
			sub := s.backend.SubscribeEvents(subCtx, test.startBlockID, test.startHeight, test.filters)

			// loop over all of the blocks
			for i, b := range s.blocks {
				execData := s.execDataMap[b.ID()]
				s.T().Logf("checking block %d %v", i, b.ID())

				// simulate new exec data received
				if i > test.highestBackfill {
					s.execDataDistributor.OnExecutionDataReceived(execData)
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
				}, 10000*time.Millisecond, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Header.Height, b.ID()))
			}

			// make sure there are no new messages waiting
			unittest.RequireNeverReturnBefore(s.T(), func() {
				// this is a failure case. the channel should be opened with nothing waiting
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

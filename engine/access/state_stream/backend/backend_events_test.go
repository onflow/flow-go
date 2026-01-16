package backend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

type BackendEventsSuite struct {
	BackendExecutionDataSuite

	chainID      flow.ChainID
	eventTypes   []flow.EventType
	eventFilters []state_stream.EventFilter
}

func TestBackendEventsSuite(t *testing.T) {
	suite.Run(t, new(BackendEventsSuite))
}

func (s *BackendEventsSuite) SetupTest() {
	s.BackendExecutionDataSuite.SetupTest()

	s.chainID = flow.MonotonicEmulator
	s.eventTypes = []flow.EventType{
		unittest.EventTypeFixture(s.chainID),
		unittest.EventTypeFixture(s.chainID),
		unittest.EventTypeFixture(s.chainID),
	}

	// empty filter; all events should be returned
	s.eventFilters = append(s.eventFilters, state_stream.EventFilter{})

	// filter for some event types; only events of those types should be returned
	filter, err := state_stream.NewEventFilter(
		state_stream.DefaultEventFilterConfig,
		s.chainID.Chain(),
		[]string{string(s.eventTypes[0])},
		nil,
		nil,
	)
	require.NoError(s.T(), err)
	s.eventFilters = append(s.eventFilters, filter)

	// filter for non-existent event types; no events should be returned
	filter, err = state_stream.NewEventFilter(
		state_stream.DefaultEventFilterConfig,
		s.chainID.Chain(),
		[]string{"A.0x1.NonExistent.Event"},
		nil,
		nil,
	)
	require.NoError(s.T(), err)
	s.eventFilters = append(s.eventFilters, filter)
}

// TestSubscribeEvents verifies that the subscription to events works correctly
// starting from the spork root block ID. It ensures that the events are received
// sequentially and matches the expected data after applying the event filter.
func (s *BackendEventsSuite) TestSubscribeEvents() {
	s.mockSubscribeFuncState()
	s.mockExecutionResultProviderStateForEvents()

	s.snapshot.
		On("Head").
		Return(s.nodeRootBlock, nil)

	s.state.
		On("AtBlockID", mock.Anything).
		Return(s.snapshot)

	s.snapshot.
		On("SealedResult").
		Return(s.executionResults[0], nil, nil)

	for _, eventFilter := range s.eventFilters {
		backend := NewEventsBackend(
			s.log,
			s.state,
			s.headers,
			s.nodeRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)
		currentHeight := s.nodeRootBlock.Height

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := backend.SubscribeEvents(
			ctx,
			s.nodeRootBlock.ID(),
			0, // either id or height must be provided
			eventFilter,
			s.criteria,
		)

		// cancel after we have received all expected blocks to avoid waiting for
		// streamer/provider to hit the "block not ready" or missing height path.
		received := 0
		expected := len(s.blocks)

		for value := range sub.Channel() {
			actualEventsResponse, ok := value.(*EventsResponse)
			require.True(s.T(), ok, "expected *EventsResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedEvents := s.extractExpectedEvents(block.ID(), eventFilter)

			require.Equal(s.T(), block.ID(), actualEventsResponse.BlockID)
			require.Equal(s.T(), block.Height, actualEventsResponse.Height)
			require.Equal(s.T(), expectedEvents, actualEventsResponse.Events)

			currentHeight += 1

			received++
			if received == expected {
				// we've validated enough; stop the stream.
				cancel()
			}
		}

		// we only break out of the above loop when the subscription's channel is closed. this happens after
		// the context cancellation is processed. At this point, the subscription should contain the error.
		require.ErrorIs(s.T(), sub.Err(), context.Canceled)
	}
}

// TestSubscribeEventsFromNonRoot verifies that subscribing with a start block
// ID different from the spork root works as expected. We start from the block right
// after the spork root and stream all remaining blocks.
func (s *BackendEventsSuite) TestSubscribeEventsFromNonRoot() {
	s.mockSubscribeFuncState()
	s.mockExecutionResultProviderStateForEvents()

	// called on the start by tracker
	startBlock := s.blocksHeightToBlockMap[s.sporkRootBlock.Height+1]

	s.snapshot.
		On("Head").
		Return(startBlock.ToHeader(), nil)

	s.state.
		On("AtBlockID", mock.Anything).
		Return(s.snapshot)

	for _, eventFilter := range s.eventFilters {
		backend := NewEventsBackend(
			s.log,
			s.state,
			s.headers,
			s.nodeRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)

		// start from the block right after the spork root
		currentHeight := startBlock.Height

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := backend.SubscribeEvents(
			ctx,
			startBlock.ID(),
			0, // either id or height must be provided
			eventFilter,
			s.criteria,
		)

		received := 0
		expected := len(s.blocks) - 1 // streaming all blocks except the spork root

		for value := range sub.Channel() {
			actualEventsResponse, ok := value.(*EventsResponse)
			require.True(s.T(), ok, "expected *EventsResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedEvents := s.extractExpectedEvents(block.ID(), eventFilter)

			require.Equal(s.T(), block.ID(), actualEventsResponse.BlockID)
			require.Equal(s.T(), block.Height, actualEventsResponse.Height)
			require.Equal(s.T(), expectedEvents, actualEventsResponse.Events)

			currentHeight += 1

			received++
			if received == expected {
				// we've validated enough; stop the stream.
				cancel()
			}
		}

		// we only break out of the above loop when the subscription's channel is closed. this happens after
		// the context cancellation is processed. At this point, the subscription should contain the error.
		require.ErrorIs(s.T(), sub.Err(), context.Canceled)
	}
}

// TestSubscribeEventsFromStartHeight verifies that the subscription can start from a specific
// block height. It ensures that the correct block header is retrieved and data streaming starts
// from the correct block.
func (s *BackendEventsSuite) TestSubscribeEventsFromStartHeight() {
	s.mockSubscribeFuncState()
	s.mockExecutionResultProviderStateForEvents()

	s.snapshot.
		On("Head").
		Return(s.nodeRootBlock, nil)

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot)

	s.state.
		On("AtBlockID", mock.Anything).
		Return(s.snapshot)

	s.snapshot.
		On("SealedResult").
		Return(s.executionResults[0], nil, nil)

	for _, eventFilter := range s.eventFilters {
		backend := NewEventsBackend(
			s.log,
			s.state,
			s.headers,
			s.nodeRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)
		currentHeight := s.sporkRootBlock.Height

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := backend.SubscribeEventsFromStartHeight(
			ctx,
			s.sporkRootBlock.Height,
			eventFilter,
			s.criteria,
		)

		// cancel after we have received all expected blocks to avoid waiting for
		// streamer/provider to hit the "block not ready" or missing height path.
		received := 0
		expected := len(s.blocks)

		for value := range sub.Channel() {
			actualEventsResponse, ok := value.(*EventsResponse)
			require.True(s.T(), ok, "expected *EventsResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedEvents := s.extractExpectedEvents(block.ID(), eventFilter)

			require.Equal(s.T(), block.ID(), actualEventsResponse.BlockID)
			require.Equal(s.T(), block.Height, actualEventsResponse.Height)
			require.Equal(s.T(), expectedEvents, actualEventsResponse.Events)

			currentHeight += 1

			received++
			if received == expected {
				// we've validated enough; stop the stream.
				cancel()
			}
		}

		// we only break out of the above loop when the subscription's channel is closed. this happens after
		// the context cancellation is processed. At this point, the subscription should contain the error.
		require.ErrorIs(s.T(), sub.Err(), context.Canceled)
	}
}

// TestSubscribeEventsFromStartID verifies that the subscription can start from a specific
// block ID. It checks that the start height is correctly resolved from the block ID and data
// streaming proceeds.
func (s *BackendEventsSuite) TestSubscribeEventsFromStartID() {
	s.mockSubscribeFuncState()
	s.mockExecutionResultProviderStateForEvents()

	s.snapshot.
		On("Head").
		Return(s.nodeRootBlock, nil)

	s.state.
		On("AtBlockID", mock.Anything).
		Return(s.snapshot)

	s.snapshot.
		On("SealedResult").
		Return(s.executionResults[0], nil, nil)

	for _, eventFilter := range s.eventFilters {
		backend := NewEventsBackend(
			s.log,
			s.state,
			s.headers,
			s.nodeRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)
		currentHeight := s.sporkRootBlock.Height

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := backend.SubscribeEventsFromStartBlockID(
			ctx,
			s.sporkRootBlock.ID(),
			eventFilter,
			s.criteria,
		)

		// cancel after we have received all expected blocks to avoid waiting for
		// streamer/provider to hit the "block not ready" or missing height path.
		received := 0
		expected := len(s.blocks)

		for value := range sub.Channel() {
			actualEventsResponse, ok := value.(*EventsResponse)
			require.True(s.T(), ok, "expected *EventsResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedEvents := s.extractExpectedEvents(block.ID(), eventFilter)

			require.Equal(s.T(), block.ID(), actualEventsResponse.BlockID)
			require.Equal(s.T(), block.Height, actualEventsResponse.Height)
			require.Equal(s.T(), expectedEvents, actualEventsResponse.Events)

			currentHeight += 1

			received++
			if received == expected {
				// we've validated enough; stop the stream.
				cancel()
			}
		}

		// we only break out of the above loop when the subscription's channel is closed. this happens after
		// the context cancellation is processed. At this point, the subscription should contain the error.
		require.ErrorIs(s.T(), sub.Err(), context.Canceled)
	}
}

// TestSubscribeEventsFromLatest verifies that the subscription can start from the latest
// available finalized block. It ensures that the start height is correctly determined and data
// streaming begins.
func (s *BackendEventsSuite) TestSubscribeEventsFromLatest() {
	s.mockSubscribeFuncState()
	s.mockExecutionResultProviderStateForEvents()

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot)

	s.state.
		On("AtBlockID", mock.Anything).
		Return(s.snapshot)

	s.state.On("Sealed").Return(s.snapshot)
	s.snapshot.On("Head").Return(s.blocks[0].ToHeader(), nil)
	s.snapshot.On("SealedResult").Return(s.executionResults[0], nil, nil)

	for _, eventFilter := range s.eventFilters {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		backend := NewEventsBackend(
			s.log,
			s.state,
			s.headers,
			s.nodeRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)

		sub := backend.SubscribeEventsFromLatest(ctx, eventFilter, s.criteria)
		currentHeight := s.sporkRootBlock.Height

		// cancel after we have received all expected blocks to avoid waiting for
		// streamer/provider to hit the "block not ready" or missing height path.
		received := 0
		expected := len(s.blocks)

		for value := range sub.Channel() {
			actualEventsResponse, ok := value.(*EventsResponse)
			require.True(s.T(), ok, "expected *EventsResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedEvents := s.extractExpectedEvents(block.ID(), eventFilter)

			require.Equal(s.T(), block.ID(), actualEventsResponse.BlockID)
			require.Equal(s.T(), block.Height, actualEventsResponse.Height)
			require.Equal(s.T(), expectedEvents, actualEventsResponse.Events)

			currentHeight += 1

			received++
			if received == expected {
				// we've validated enough; stop the stream.
				cancel()
			}
		}

		// we only break out of the above loop when the subscription's channel is closed. this happens after
		// the context cancellation is processed. At this point, the subscription should contain the error.
		require.ErrorIs(s.T(), sub.Err(), context.Canceled)
	}
}

// mockExecutionResultProviderState sets up mock expectations for the code that calls the execution result provider.
func (s *BackendExecutionDataSuite) mockExecutionResultProviderStateForEvents() {
	s.snapshot.
		On("Identities", mock.Anything).
		Return(s.fixedExecutionNodes, nil)

	s.eventsReader.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) ([]flow.Event, error) {
			events, ok := s.blockIDToEventsMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}

			return events, nil
		})

	s.executionDataSnapshot.
		On("Events").
		Return(s.eventsReader)

	s.executionStateCache.
		On("Snapshot", mock.Anything).
		Return(s.executionDataSnapshot, nil)
}

// extractExpectedEvents extracts events from execution data and applies the filter.
func (s *BackendEventsSuite) extractExpectedEvents(blockID flow.Identifier, filter state_stream.EventFilter) flow.EventsList {
	events := s.blockIDToEventsMap[blockID]
	if events == nil {
		return nil
	}

	return filter.Filter(events)
}

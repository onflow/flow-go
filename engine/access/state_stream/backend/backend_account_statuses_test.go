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
)

type BackendAccountStatusesSuite struct {
	BackendExecutionDataSuite

	chainID       flow.ChainID
	statusFilters []state_stream.AccountStatusFilter
}

func TestBackendAccountStatusesSuite2(t *testing.T) {
	suite.Run(t, new(BackendAccountStatusesSuite))
}

func (s *BackendAccountStatusesSuite) SetupTest() {
	s.BackendExecutionDataSuite.SetupTest()

	s.chainID = flow.MonotonicEmulator

	// empty filter (no event types, no addresses); all account statuses should be returned
	filter, err := state_stream.NewAccountStatusFilter(
		state_stream.DefaultEventFilterConfig,
		s.chainID.Chain(),
		[]string{},
		[]string{},
	)
	require.NoError(s.T(), err)
	s.statusFilters = append(s.statusFilters, filter)
}

// extractExpectedAccountStatuses extracts events from execution data, applies the filter,
// and groups them by account address.
func (s *BackendAccountStatusesSuite) extractExpectedAccountStatuses(blockID flow.Identifier, filter state_stream.AccountStatusFilter) map[string]flow.EventsList {
	execData := s.blockIDToExecutionDataMap[blockID]
	if execData == nil {
		return nil
	}

	var events flow.EventsList
	for _, chunkExecutionData := range execData.ChunkExecutionDatas {
		events = append(events, chunkExecutionData.Events...)
	}

	filteredProtocolEvents := filter.Filter(events)
	return filter.GroupCoreEventsByAccountAddress(filteredProtocolEvents, s.log)
}

// TestSubscribeAccountStatuses verifies that the subscription to account statuses works correctly
// starting from the spork root block ID. It ensures that the account statuses are received
// sequentially and matches the expected data after applying the status filter.
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatuses() {
	s.mockSubscribeFuncState()

	s.headers.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			block, ok := s.blocksIDToBlockMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}
			return block.ToHeader(), nil
		})

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	for _, statusFilter := range s.statusFilters {
		backend := NewAccountStatusesBackend(
			s.log,
			s.state,
			s.headers,
			s.sporkRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)
		currentHeight := s.sporkRootBlock.Height

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := backend.SubscribeAccountStatuses(
			ctx,
			s.sporkRootBlock.ID(),
			0, // either id or height must be provided
			statusFilter,
			s.criteria,
		)

		// cancel after we have received all expected blocks to avoid waiting for
		// streamer/provider to hit the "block not ready" or missing height path.
		received := 0
		expected := len(s.blocks)

		for value := range sub.Channel() {
			actualResponse, ok := value.(*AccountStatusesResponse)
			require.True(s.T(), ok, "expected *AccountStatusesResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedAccountEvents := s.extractExpectedAccountStatuses(block.ID(), statusFilter)

			require.Equal(s.T(), block.ID(), actualResponse.BlockID)
			require.Equal(s.T(), block.Height, actualResponse.Height)
			require.Equal(s.T(), expectedAccountEvents, actualResponse.AccountEvents)

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

// TestSubscribeAccountStatusesFromNonRoot verifies that subscribing with a start block
// ID different from the spork root works as expected. We start from the block right
// after the spork root and stream all remaining blocks.
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatusesFromNonRoot() {
	s.mockSubscribeFuncState()

	s.headers.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			block, ok := s.blocksIDToBlockMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}
			return block.ToHeader(), nil
		})

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	// called on the start by tracker
	startBlock := s.blocksHeightToBlockMap[s.sporkRootBlock.Height+1]

	for _, statusFilter := range s.statusFilters {
		backend := NewAccountStatusesBackend(
			s.log,
			s.state,
			s.headers,
			s.sporkRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)

		// start from the block right after the spork root
		currentHeight := startBlock.Height

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := backend.SubscribeAccountStatuses(
			ctx,
			startBlock.ID(),
			0, // either id or height must be provided
			statusFilter,
			s.criteria,
		)

		received := 0
		expected := len(s.blocks) - 1 // streaming all blocks except the spork root

		for value := range sub.Channel() {
			actualResponse, ok := value.(*AccountStatusesResponse)
			require.True(s.T(), ok, "expected *AccountStatusesResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedAccountEvents := s.extractExpectedAccountStatuses(block.ID(), statusFilter)

			require.Equal(s.T(), block.ID(), actualResponse.BlockID)
			require.Equal(s.T(), block.Height, actualResponse.Height)
			require.Equal(s.T(), expectedAccountEvents, actualResponse.AccountEvents)

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

// TestSubscribeAccountStatusesFromStartHeight verifies that the subscription can start from a specific
// block height. It ensures that the correct block header is retrieved and data streaming starts
// from the correct block.
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatusesFromStartHeight() {
	s.mockSubscribeFuncState()

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	s.headers.
		On("ByHeight", s.sporkRootBlock.Height).
		Return(s.sporkRootBlock.ToHeader(), nil)

	for _, statusFilter := range s.statusFilters {
		backend := NewAccountStatusesBackend(
			s.log,
			s.state,
			s.headers,
			s.sporkRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)
		currentHeight := s.sporkRootBlock.Height

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := backend.SubscribeAccountStatusesFromStartHeight(
			ctx,
			s.sporkRootBlock.Height,
			statusFilter,
			s.criteria,
		)

		// cancel after we have received all expected blocks to avoid waiting for
		// streamer/provider to hit the "block not ready" or missing height path.
		received := 0
		expected := len(s.blocks)

		for value := range sub.Channel() {
			actualResponse, ok := value.(*AccountStatusesResponse)
			require.True(s.T(), ok, "expected *AccountStatusesResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedAccountEvents := s.extractExpectedAccountStatuses(block.ID(), statusFilter)

			require.Equal(s.T(), block.ID(), actualResponse.BlockID)
			require.Equal(s.T(), block.Height, actualResponse.Height)
			require.Equal(s.T(), expectedAccountEvents, actualResponse.AccountEvents)

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

// TestSubscribeAccountStatusesFromStartID verifies that the subscription can start from a specific
// block ID. It checks that the start height is correctly resolved from the block ID and data
// streaming proceeds.
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatusesFromStartID() {
	s.mockSubscribeFuncState()

	s.headers.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			block, ok := s.blocksIDToBlockMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}
			return block.ToHeader(), nil
		})

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	for _, statusFilter := range s.statusFilters {
		backend := NewAccountStatusesBackend(
			s.log,
			s.state,
			s.headers,
			s.sporkRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)
		currentHeight := s.sporkRootBlock.Height

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := backend.SubscribeAccountStatusesFromStartBlockID(
			ctx,
			s.sporkRootBlock.ID(),
			statusFilter,
			s.criteria,
		)

		// cancel after we have received all expected blocks to avoid waiting for
		// streamer/provider to hit the "block not ready" or missing height path.
		received := 0
		expected := len(s.blocks)

		for value := range sub.Channel() {
			actualResponse, ok := value.(*AccountStatusesResponse)
			require.True(s.T(), ok, "expected *AccountStatusesResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedAccountEvents := s.extractExpectedAccountStatuses(block.ID(), statusFilter)

			require.Equal(s.T(), block.ID(), actualResponse.BlockID)
			require.Equal(s.T(), block.Height, actualResponse.Height)
			require.Equal(s.T(), expectedAccountEvents, actualResponse.AccountEvents)

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

// TestSubscribeAccountStatusesFromLatest verifies that the subscription can start from the latest
// available finalized block. It ensures that the start height is correctly determined and data
// streaming begins.
func (s *BackendAccountStatusesSuite) TestSubscribeAccountStatusesFromLatest() {
	s.mockSubscribeFuncState()

	s.headers.
		On("ByHeight", s.sporkRootBlock.Height).
		Return(s.sporkRootBlock.ToHeader(), nil)

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	s.state.On("Sealed").Return(s.snapshot, nil)
	s.snapshot.On("Head").Return(s.blocks[0].ToHeader(), nil)

	for _, statusFilter := range s.statusFilters {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		backend := NewAccountStatusesBackend(
			s.log,
			s.state,
			s.headers,
			s.sporkRootBlock,
			s.executionDataTracker,
			s.executionResultProvider,
			s.executionStateCache,
			s.subscriptionFactory,
		)

		sub := backend.SubscribeAccountStatusesFromLatestBlock(ctx, statusFilter, s.criteria)
		currentHeight := s.sporkRootBlock.Height

		// cancel after we have received all expected blocks to avoid waiting for
		// streamer/provider to hit the "block not ready" or missing height path.
		received := 0
		expected := len(s.blocks)

		for value := range sub.Channel() {
			actualResponse, ok := value.(*AccountStatusesResponse)
			require.True(s.T(), ok, "expected *AccountStatusesResponse on the channel")

			block := s.blocksHeightToBlockMap[currentHeight]
			expectedAccountEvents := s.extractExpectedAccountStatuses(block.ID(), statusFilter)

			require.Equal(s.T(), block.ID(), actualResponse.BlockID)
			require.Equal(s.T(), block.Height, actualResponse.Height)
			require.Equal(s.T(), expectedAccountEvents, actualResponse.AccountEvents)

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

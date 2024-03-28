package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type EventsResponse struct {
	BlockID        flow.Identifier
	Height         uint64
	Events         flow.EventsList
	BlockTimestamp time.Time
	MessageIndex   uint64
}

type EventsBackend struct {
	log                 zerolog.Logger
	subscriptionHandler *subscription.SubscriptionHandler

	headers          storage.Headers
	useIndex         bool
	eventsIndex      *index.EventsIndex
	getExecutionData GetExecutionDataFunc

	executionDataTracker subscription.ExecutionDataTracker
}

// SubscribeEvents streams events for all blocks starting at the specified block ID or block height
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Only one of startBlockID and startHeight may be set. If neither startBlockID nor startHeight is provided,
// the latest sealed block is used.
//
// Events within each block are filtered by the provided EventFilter, and only
// those events that match the filter are returned. If no filter is provided,
// all events are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
// - filter: The event filter used to filter events.
//
// If invalid parameters will be supplied SubscribeEvents will return a failed subscription.
//
// Method will be deprecated. Use SubscribeEventsFromStartBlockID, SubscribeEventsFromStartHeight or SubscribeEventsFromLatest.
func (b *EventsBackend) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	messageIndex := counters.NewMonotonousCounter(0)
	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponseFactory(filter, &messageIndex))
}

// SubscribeEventsFromStartBlockID streams events starting at the specified block ID,
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Events within each block are filtered by the provided EventFilter, and only
// those events that match the filter are returned. If no filter is provided,
// all events are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block.
// - filter: The event filter used to filter events.
//
// If invalid parameters will be supplied SubscribeEventsFromStartBlockID will return a failed subscription.
func (b *EventsBackend) SubscribeEventsFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, filter state_stream.EventFilter) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block id")
	}

	return b.subscribeEvents(ctx, nextHeight, filter)
}

// SubscribeEventsFromStartHeight streams events starting at the specified block height,
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Events within each block are filtered by the provided EventFilter, and only
// those events that match the filter are returned. If no filter is provided,
// all events are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startHeight: The height of the starting block.
// - filter: The event filter used to filter events.
//
// If invalid parameters will be supplied SubscribeEventsFromStartHeight will return a failed subscription.
func (b *EventsBackend) SubscribeEventsFromStartHeight(ctx context.Context, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromHeight(startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}

	return b.subscribeEvents(ctx, nextHeight, filter)
}

// SubscribeEventsFromLatest subscribes to events starting at the latest sealed block,
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Events within each block are filtered by the provided EventFilter, and only
// those events that match the filter are returned. If no filter is provided,
// all events are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - filter: The event filter used to filter events.
//
// If invalid parameters will be supplied SubscribeEventsFromLatest will return a failed subscription.
func (b *EventsBackend) SubscribeEventsFromLatest(ctx context.Context, filter state_stream.EventFilter) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}

	return b.subscribeEvents(ctx, nextHeight, filter)
}

// subscribeEvents is a helper function that subscribes to events starting at the specified height,
// filtered by the provided event filter.
//
// Parameters:
// - ctx: Context for the operation.
// - nextHeight: The height of the starting block.
// - filter: The event filter used to filter events.
//
// No errors are expected during normal operation.
func (b *EventsBackend) subscribeEvents(ctx context.Context, nextHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
	messageIndex := counters.NewMonotonousCounter(0)
	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponseFactory(filter, &messageIndex))
}

// getResponseFactory returns a function that retrieves the event response for a given height.
//
// Parameters:
// - filter: The event filter used to filter events.
// - index: A strict monotonous counter used to track the message index.
//
// Expected errors during normal operation:
// - codes.NotFound: If block header for the specified block height is not found, if events for the specified block height are not found.
// - codes.Internal: If the message index has already been incremented.
func (b *EventsBackend) getResponseFactory(filter state_stream.EventFilter, index *counters.StrictMonotonousCounter) subscription.GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		var response *EventsResponse
		var header *flow.Header
		var err error

		if b.useIndex {
			response, err = b.getEventsFromStorage(height, filter)
		} else {
			response, err = b.getEventsFromExecutionData(ctx, height, filter)
		}

		if err == nil {
			header, err = b.headers.ByHeight(height)
			if err != nil {
				return nil, rpc.ConvertStorageError(err)
			}

			response.BlockTimestamp = header.Timestamp

			messageIndex := index.Value()
			if ok := index.Set(messageIndex + 1); !ok {
				return nil, status.Errorf(codes.Internal, "the message index has already been incremented to %d", index.Value())
			}
			response.MessageIndex = messageIndex

			if b.log.GetLevel() == zerolog.TraceLevel {
				b.log.Trace().
					Hex("block_id", logging.ID(response.BlockID)).
					Uint64("height", height).
					Int("events", len(response.Events)).
					Msg("sending events")
			}
		}
		return response, err
	}
}

// getEventsFromExecutionData retrieves the events for a given height extracted from the execution data.
//
// Parameters:
// - ctx: Context for the operation.
// - height: The height of the block for which events are retrieved.
// - filter: The event filter used to filter events.
//
// Expected errors during normal operation:
//   - codes.NotFound: If block header for the specified block height is not found, if events for the specified block height are not found.
func (b *EventsBackend) getEventsFromExecutionData(ctx context.Context, height uint64, filter state_stream.EventFilter) (*EventsResponse, error) {
	executionData, err := b.getExecutionData(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	var events flow.EventsList
	for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
		events = append(events, filter.Filter(chunkExecutionData.Events)...)
	}

	return &EventsResponse{
		BlockID: executionData.BlockID,
		Height:  height,
		Events:  events,
	}, nil
}

// getEventsFromStorage retrieves the events for a given height from the index storage.
//
// Parameters:
// - height: The height of the block for which events are retrieved.
// - filter: The event filter used to filter events.
//
// Expected errors during normal operation:
//   - codes.NotFound: If block header for the specified block height is not found, if events for the specified block height are not found.
func (b *EventsBackend) getEventsFromStorage(height uint64, filter state_stream.EventFilter) (*EventsResponse, error) {
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}

	events, err := b.eventsIndex.ByBlockID(blockID, height)
	if err != nil {
		return nil, fmt.Errorf("could not get events for block %d: %w", height, err)
	}

	b.log.Trace().
		Uint64("height", height).
		Hex("block_id", logging.ID(blockID)).
		Int("events", len(events)).
		Msg("events from storage")

	return &EventsResponse{
		BlockID: blockID,
		Height:  height,
		Events:  filter.Filter(events),
	}, nil
}

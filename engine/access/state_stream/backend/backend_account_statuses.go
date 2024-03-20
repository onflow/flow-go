package backend

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
)

type AccountStatusesResponse struct {
	BlockID       flow.Identifier
	Height        uint64
	AccountEvents map[string]flow.EventsList
	MessageIndex  uint64
}

// AccountStatusesBackend is a struct representing a backend implementation for subscribing to account statuses changes.
type AccountStatusesBackend struct {
	log            zerolog.Logger
	broadcaster    *engine.Broadcaster
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	executionDataTracker subscription.ExecutionDataTracker
	eventsRetriever      EventsRetriever
}

// subscribe creates and returns a subscription to receive account status updates starting from the specified height.
func (b *AccountStatusesBackend) subscribe(
	ctx context.Context,
	nextHeight uint64,
	filter state_stream.AccountStatusFilter,
) subscription.Subscription {
	messageIndex := counters.NewMonotonousCounter(0)
	sub := subscription.NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getAccountStatusResponseFactory(&messageIndex, filter))
	go subscription.NewStreamer(b.log, b.broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

// SubscribeAccountStatusesFromStartBlockID subscribes to the streaming of account status changes starting from
// a specific block ID with an optional status filter.
// Errors:
// - codes.ErrNotFound if could not get block by start blockID.
// - codes.Internal if there is an internal error.
func (b *AccountStatusesBackend) SubscribeAccountStatusesFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	filter state_stream.AccountStatusFilter,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block id")
	}
	return b.subscribe(ctx, nextHeight, filter)
}

// SubscribeAccountStatusesFromStartHeight subscribes to the streaming of account status changes starting from
// a specific block height, with an optional status filter.
// Errors:
// - codes.ErrNotFound if could not get block by  start height.
// - codes.Internal if there is an internal error.
func (b *AccountStatusesBackend) SubscribeAccountStatusesFromStartHeight(
	ctx context.Context,
	startHeight uint64,
	filter state_stream.AccountStatusFilter,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromHeight(startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}
	return b.subscribe(ctx, nextHeight, filter)
}

// SubscribeAccountStatusesFromLatestBlock subscribes to the streaming of account status changes starting from a
// latest sealed block, with an optional status filter.
//
// No errors are expected during normal operation.
func (b *AccountStatusesBackend) SubscribeAccountStatusesFromLatestBlock(
	ctx context.Context,
	filter state_stream.AccountStatusFilter,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from latest")
	}
	return b.subscribe(ctx, nextHeight, filter)
}

// getAccountStatusResponseFactory returns a function that returns the account statuses response for a given height.
func (b *AccountStatusesBackend) getAccountStatusResponseFactory(
	messageIndex *counters.StrictMonotonousCounter,
	filter state_stream.AccountStatusFilter,
) subscription.GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		var err error
		eventsResponse, err := b.eventsRetriever.GetAllEventsResponse(ctx, height)
		if err != nil {
			return nil, err
		}
		filteredProtocolEvents := filter.Filter(eventsResponse.Events)
		allAccountProtocolEvents := filter.CreateAccountRelatedCoreEvents(filteredProtocolEvents, b.log)

		index := messageIndex.Value()
		if ok := messageIndex.Set(index + 1); !ok {
			return nil, status.Errorf(codes.Internal, "message index already incremented to %d", messageIndex.Value())
		}
		return &AccountStatusesResponse{
			BlockID:       eventsResponse.BlockID,
			Height:        eventsResponse.Height,
			AccountEvents: allAccountProtocolEvents,
			MessageIndex:  index,
		}, nil
	}
}

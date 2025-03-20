package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
)

// eventsArguments contains the arguments a user passes to subscribe to events
type eventsArguments struct {
	StartBlockID      flow.Identifier          // ID of the block to start subscription from
	StartBlockHeight  uint64                   // Height of the block to start subscription from
	Filter            state_stream.EventFilter // Filter applied to events for a given subscription
	HeartbeatInterval uint64                   // Maximum number of blocks message won't be sent
}

// EventsDataProvider is responsible for providing events
type EventsDataProvider struct {
	*baseDataProvider

	stateStreamApi         state_stream.API
	arguments              eventsArguments
	messageIndex           counters.StrictMonotonicCounter
	blocksSinceLastMessage uint64
}

var _ DataProvider = (*EventsDataProvider)(nil)

// NewEventsDataProvider creates a new instance of EventsDataProvider.
func NewEventsDataProvider(
	logger zerolog.Logger,
	stateStreamApi state_stream.API,
	subscriptionID string,
	topic string,
	rawArguments wsmodels.Arguments,
	send chan<- interface{},
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
	defaultHeartbeatInterval uint64,
) (*EventsDataProvider, error) {
	if stateStreamApi == nil {
		return nil, fmt.Errorf("this access node does not support streaming events")
	}

	args, err := parseEventsArguments(rawArguments, chain, eventFilterConfig, defaultHeartbeatInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for events data provider: %w", err)
	}

	provider := newBaseDataProvider(
		logger.With().Str("component", "events-data-provider").Logger(),
		nil,
		subscriptionID,
		topic,
		rawArguments,
		send,
	)

	return &EventsDataProvider{
		baseDataProvider:       provider,
		stateStreamApi:         stateStreamApi,
		arguments:              args,
		messageIndex:           counters.NewMonotonicCounter(0),
		blocksSinceLastMessage: 0,
	}, nil
}

// Run starts processing the subscription for events and handles responses.
// Must be called once.
//
// No errors expected during normal operations
func (p *EventsDataProvider) Run(ctx context.Context) error {
	// we read data from the subscription and send them to client's channel
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p.subscriptionState = newSubscriptionState(cancel, p.createAndStartSubscription(ctx, p.arguments))

	return run(
		p.subscriptionState.subscription,
		func(response *backend.EventsResponse) error {
			return p.sendResponse(response)
		},
	)
}

// sendResponse processes an event message and sends it to client's channel.
// This function is not expected to be called concurrently.
//
// No errors are expected during normal operations.
func (p *EventsDataProvider) sendResponse(eventsResponse *backend.EventsResponse) error {
	// Only send a response if there's meaningful data to send
	// or the heartbeat interval limit is reached
	p.blocksSinceLastMessage += 1
	contractEmittedEvents := len(eventsResponse.Events) != 0
	reachedHeartbeatLimit := p.blocksSinceLastMessage >= p.arguments.HeartbeatInterval
	if !contractEmittedEvents && !reachedHeartbeatLimit {
		return nil
	}

	eventsPayload := models.NewEventResponse(eventsResponse, p.messageIndex.Value())
	response := models.BaseDataProvidersResponse{
		SubscriptionID: p.ID(),
		Topic:          p.Topic(),
		Payload:        eventsPayload,
	}
	p.send <- &response

	p.blocksSinceLastMessage = 0
	p.messageIndex.Increment()

	return nil
}

// createAndStartSubscription creates a new subscription using the specified input arguments.
func (p *EventsDataProvider) createAndStartSubscription(ctx context.Context, args eventsArguments) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.stateStreamApi.SubscribeEventsFromStartBlockID(ctx, args.StartBlockID, args.Filter)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.stateStreamApi.SubscribeEventsFromStartHeight(ctx, args.StartBlockHeight, args.Filter)
	}

	return p.stateStreamApi.SubscribeEventsFromLatest(ctx, args.Filter)
}

// parseEventsArguments validates and initializes the events arguments.
func parseEventsArguments(
	arguments wsmodels.Arguments,
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
	defaultHeartbeatInterval uint64,
) (eventsArguments, error) {
	allowedFields := map[string]struct{}{
		"start_block_id":     {},
		"start_block_height": {},
		"event_types":        {},
		"addresses":          {},
		"contracts":          {},
		"heartbeat_interval": {},
	}
	err := ensureAllowedFields(arguments, allowedFields)
	if err != nil {
		return eventsArguments{}, err
	}

	var args eventsArguments

	// Parse block arguments
	startBlockID, startBlockHeight, err := parseStartBlock(arguments)
	if err != nil {
		return eventsArguments{}, err
	}
	args.StartBlockID = startBlockID
	args.StartBlockHeight = startBlockHeight

	// Parse 'heartbeat_interval' argument
	heartbeatInterval, err := extractHeartbeatInterval(arguments, defaultHeartbeatInterval)
	if err != nil {
		return eventsArguments{}, err
	}
	args.HeartbeatInterval = heartbeatInterval

	// Parse 'event_types' as a JSON array
	eventTypes, err := extractArrayOfStrings(arguments, "event_types", false)
	if err != nil {
		return eventsArguments{}, err
	}

	// Parse 'addresses' as []string{}
	addresses, err := extractArrayOfStrings(arguments, "addresses", false)
	if err != nil {
		return eventsArguments{}, err
	}

	// Parse 'contracts' as []string{}
	contracts, err := extractArrayOfStrings(arguments, "contracts", false)
	if err != nil {
		return eventsArguments{}, err
	}

	// Initialize the event filter with the parsed arguments
	args.Filter, err = state_stream.NewEventFilter(eventFilterConfig, chain, eventTypes, addresses, contracts)
	if err != nil {
		return eventsArguments{}, fmt.Errorf("error creating event filter: %w", err)
	}

	return args, nil
}

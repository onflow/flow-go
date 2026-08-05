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
	"github.com/onflow/flow-go/engine/common/rpc/convert"
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
	ctx context.Context,
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
		ctx,
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
func (p *EventsDataProvider) Run() error {
	return run(
		p.createAndStartSubscription(p.ctx, p.arguments),
		p.handleResponse,
	)
}

// handleResponse processes the response from the subscription and sends it to the client's channel.
// As part of the processing, it converts the event payloads from CCF to JSON-CDC format.
// This function is not expected to be called concurrently.
//
// No errors expected during normal operations.
func (p *EventsDataProvider) handleResponse(response *backend.EventsResponse) error {
	// convert events to JSON-CDC format
	convertedResponse, err := convertEventsResponse(response)
	if err != nil {
		return fmt.Errorf("failed to convert events to JSON-CDC format: %w", err)
	}

	return p.sendResponse(convertedResponse)
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

// convertEventsResponse converts events in the provided EventsResponse from CCF to JSON-CDC format.
//
// No errors expected during normal operations.
func convertEventsResponse(resp *backend.EventsResponse) (*backend.EventsResponse, error) {
	jsoncdcEvents, err := convertEvents(resp.Events)
	if err != nil {
		return nil, fmt.Errorf("failed to convert events to JSON-CDC: %w", err)
	}

	return &backend.EventsResponse{
		BlockID:        resp.BlockID,
		Height:         resp.Height,
		BlockTimestamp: resp.BlockTimestamp,
		Events:         jsoncdcEvents,
	}, nil
}

// convertEvents converts a slice events with CCF encoded payloads into a slice of new events who's
// payloads are encoded in JSON-CDC format.
//
// Note: this function creates a copy of the original events before converting the payload. This
// is important to ensure the original data structure is not modified, which could impact data held
// in caches.
//
// No errors expected during normal operations.
func convertEvents(ccfEvents []flow.Event) ([]flow.Event, error) {
	jsoncdcEvents := make([]flow.Event, len(ccfEvents))
	for i, ccfEvent := range ccfEvents {
		converted, err := convert.CcfEventToJsonEvent(ccfEvent)
		if err != nil {
			return nil, fmt.Errorf("failed to convert event %d: %w", i, err)
		}
		jsoncdcEvents[i] = *converted
	}
	return jsoncdcEvents, nil
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

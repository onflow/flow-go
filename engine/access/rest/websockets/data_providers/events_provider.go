package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
)

// EventsArguments contains the arguments required for subscribing to events
type EventsArguments struct {
	StartBlockID     flow.Identifier          // ID of the block to start subscription from
	StartBlockHeight uint64                   // Height of the block to start subscription from
	Filter           state_stream.EventFilter // Filter applied to events for a given subscription
}

// EventsDataProvider is responsible for providing events
type EventsDataProvider struct {
	*BaseDataProviderImpl

	logger         zerolog.Logger
	args           EventsArguments
	stateStreamApi state_stream.API
}

var _ DataProvider = (*EventsDataProvider)(nil)

// NewEventsDataProvider creates a new instance of EventsDataProvider.
func NewEventsDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	stateStreamApi state_stream.API,
	topic string,
	arguments map[string]string,
	send chan<- interface{},
) (*EventsDataProvider, error) {
	p := &EventsDataProvider{
		logger:         logger.With().Str("component", "events-data-provider").Logger(),
		stateStreamApi: stateStreamApi,
	}

	// Initialize arguments passed to the provider.
	var err error
	p.args, err = ParseEventsArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for events data provider: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	// Set up a subscription to events based on arguments.
	sub := p.createSubscription(subCtx)

	p.BaseDataProviderImpl = NewBaseDataProviderImpl(
		cancel,
		topic,
		send,
		sub,
	)

	return p, nil
}

// Run starts processing the subscription for events and handles responses.
//
// No errors are expected during normal operations.
func (p *EventsDataProvider) Run() error {
	return subscription.HandleSubscription(p.subscription, p.handleResponse(p.send))
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *EventsDataProvider) createSubscription(ctx context.Context) subscription.Subscription {
	if p.args.StartBlockID != flow.ZeroID && p.args.StartBlockHeight != request.EmptyHeight {
		return p.stateStreamApi.SubscribeEvents(ctx, p.args.StartBlockID, p.args.StartBlockHeight, p.args.Filter)
	}

	if p.args.StartBlockID != flow.ZeroID {
		return p.stateStreamApi.SubscribeEventsFromStartBlockID(ctx, p.args.StartBlockID, p.args.Filter)
	}

	if p.args.StartBlockHeight != request.EmptyHeight {
		return p.stateStreamApi.SubscribeEventsFromStartHeight(ctx, p.args.StartBlockHeight, p.args.Filter)
	}

	return p.stateStreamApi.SubscribeEventsFromLatest(ctx, p.args.Filter)
}

// handleResponse processes an event and sends the formatted response.
//
// No errors are expected during normal operations.
func (p *EventsDataProvider) handleResponse(send chan<- interface{}) func(*flow.Event) error {
	return func(event *flow.Event) error {
		send <- &models.EventResponse{
			Event: event,
		}

		return nil
	}
}

// ParseEventsArguments validates and initializes the events arguments.
func ParseEventsArguments(arguments map[string]string) (EventsArguments, error) {
	var args EventsArguments

	// Parse
	if eventStatusIn, ok := arguments["event_filter"]; ok {
		eventFilter := parser.ParseEventFilter(eventStatusIn)
		if err != nil {
			return args, err
		}
		args.Filter = eventFilter
	} else {
		return args, fmt.Errorf("'event_filter' must be provided")
	}

	// Parse 'start_block_id' if provided
	if startBlockIDIn, ok := arguments["start_block_id"]; ok {
		var startBlockID parser.ID
		err := startBlockID.Parse(startBlockIDIn)
		if err != nil {
			return args, err
		}
		args.StartBlockID = startBlockID.Flow()
	}

	// Parse 'start_block_height' if provided
	if startBlockHeightIn, ok := arguments["start_block_height"]; ok {
		var err error
		args.StartBlockHeight, err = util.ToUint64(startBlockHeightIn)
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_height': %w", err)
		}
	} else {
		args.StartBlockHeight = request.EmptyHeight
	}

	return args, nil
}

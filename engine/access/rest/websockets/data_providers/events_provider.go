package data_providers

import (
	"context"
	"fmt"
	"strings"

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
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
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
	p.args, err = ParseEventsArguments(arguments, chain, eventFilterConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for events data provider: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	// Set up a subscription to events based on arguments.
	sub := p.createSubscription(subCtx)

	p.BaseDataProviderImpl = NewBaseDataProviderImpl(
		topic,
		cancel,
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
func ParseEventsArguments(
	arguments map[string]string,
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
) (EventsArguments, error) {
	var args EventsArguments

	// Parse 'event_types' as []string{}
	var eventTypes []string
	if eventTypesIn, ok := arguments["event_types"]; ok {
		if eventTypesIn != "" {
			eventTypes = strings.Split(eventTypesIn, ",")
		}
	}

	// Parse 'addresses' as []string{}
	var addresses []string
	if addressesIn, ok := arguments["addresses"]; ok {
		if addressesIn != "" {
			addresses = strings.Split(addressesIn, ",")
		}
	}

	// Parse 'contracts' as []string{}
	var contracts []string
	if contractsIn, ok := arguments["contracts"]; ok {
		if contractsIn != "" {
			contracts = strings.Split(contractsIn, ",")
		}
	}

	// Initialize the event filter with the parsed arguments
	filter, err := state_stream.NewEventFilter(eventFilterConfig, chain, eventTypes, addresses, contracts)
	if err != nil {
		return args, err
	}
	args.Filter = filter

	// Parse 'start_block_id' if provided
	if startBlockIDIn, ok := arguments["start_block_id"]; ok {
		var startBlockID parser.ID
		err = startBlockID.Parse(startBlockIDIn)
		if err != nil {
			return args, err
		}
		args.StartBlockID = startBlockID.Flow()
	}

	// Parse 'start_block_height' if provided
	if startBlockHeightIn, ok := arguments["start_block_height"]; ok {
		args.StartBlockHeight, err = util.ToUint64(startBlockHeightIn)
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_height': %w", err)
		}
	} else {
		args.StartBlockHeight = request.EmptyHeight
	}

	return args, nil
}

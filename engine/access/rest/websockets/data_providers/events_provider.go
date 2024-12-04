package data_providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
)

// EventsArguments contains the arguments required for subscribing to events
type EventsArguments struct {
	StartBlockID     flow.Identifier          // ID of the block to start subscription from
	StartBlockHeight uint64                   // Height of the block to start subscription from
	Filter           state_stream.EventFilter // Filter applied to events for a given subscription
}

// EventsDataProvider is responsible for providing events
type EventsDataProvider struct {
	*baseDataProvider

	logger         zerolog.Logger
	stateStreamApi state_stream.API
}

var _ DataProvider = (*EventsDataProvider)(nil)

// NewEventsDataProvider creates a new instance of EventsDataProvider.
func NewEventsDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	stateStreamApi state_stream.API,
	topic string,
	arguments models.Arguments,
	send chan<- interface{},
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
) (*EventsDataProvider, error) {
	p := &EventsDataProvider{
		logger:         logger.With().Str("component", "events-data-provider").Logger(),
		stateStreamApi: stateStreamApi,
	}

	// Initialize arguments passed to the provider.
	eventArgs, err := ParseEventsArguments(arguments, chain, eventFilterConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for events data provider: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	p.baseDataProvider = newBaseDataProvider(
		topic,
		cancel,
		send,
		p.createSubscription(subCtx, eventArgs), // Set up a subscription to events based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for events and handles responses.
//
// No errors are expected during normal operations.
func (p *EventsDataProvider) Run() error {
	messageIndex := counters.NewMonotonousCounter(1)

	return subscription.HandleSubscription(
		p.subscription,
		subscription.HandleResponse(p.send, func(event *flow.Event) (interface{}, error) {
			index := messageIndex.Value()
			if ok := messageIndex.Set(messageIndex.Value() + 1); !ok {
				return nil, status.Errorf(codes.Internal, "message index already incremented to %d", messageIndex.Value())
			}

			return &models.EventResponse{
				Event:        event,
				MessageIndex: strconv.FormatUint(index, 10),
			}, nil
		}))

}

// createSubscription creates a new subscription using the specified input arguments.
func (p *EventsDataProvider) createSubscription(ctx context.Context, args EventsArguments) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.stateStreamApi.SubscribeEventsFromStartBlockID(ctx, args.StartBlockID, args.Filter)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.stateStreamApi.SubscribeEventsFromStartHeight(ctx, args.StartBlockHeight, args.Filter)
	}

	return p.stateStreamApi.SubscribeEventsFromLatest(ctx, args.Filter)
}

// ParseEventsArguments validates and initializes the events arguments.
func ParseEventsArguments(
	arguments models.Arguments,
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
) (EventsArguments, error) {
	var args EventsArguments

	// Check for mutual exclusivity of start_block_id and start_block_height early
	startBlockIDIn, hasStartBlockID := arguments["start_block_id"]
	startBlockHeightIn, hasStartBlockHeight := arguments["start_block_height"]

	if hasStartBlockID && hasStartBlockHeight {
		return args, fmt.Errorf("can only provide either 'start_block_id' or 'start_block_height'")
	}

	// Parse 'start_block_id' if provided
	if hasStartBlockID {
		var startBlockID parser.ID
		err := startBlockID.Parse(startBlockIDIn)
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_id': %w", err)
		}
		args.StartBlockID = startBlockID.Flow()
	}

	// Parse 'start_block_height' if provided
	if hasStartBlockHeight {
		var err error
		args.StartBlockHeight, err = util.ToUint64(startBlockHeightIn)
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_height': %w", err)
		}
	} else {
		args.StartBlockHeight = request.EmptyHeight
	}

	var eventTypes parser.EventTypes
	// Parse 'event_types' as []string{}
	if eventTypesIn, ok := arguments["event_types"]; ok && eventTypesIn != "" {
		err := json.Unmarshal([]byte(eventTypesIn), &eventTypes) // Expect a JSON array
		if err != nil {
			return args, fmt.Errorf("could not parse 'event_types': %w", err)
		}

		err = eventTypes.Parse(strings.Split(eventTypesIn, ","))
		if err != nil {
			return args, fmt.Errorf("invalid 'event_types': %w", err)
		}
	}

	// Parse 'addresses' as []string{}
	var addresses []string
	if addressesIn, ok := arguments["addresses"]; ok && addressesIn != "" {
		err := json.Unmarshal([]byte(addressesIn), &addresses) // Expect a JSON array
		if err != nil {
			return args, fmt.Errorf("could not parse 'addresses': %w", err)
		}
	}

	// Parse 'contracts' as []string{}
	var contracts []string
	if contractsIn, ok := arguments["contracts"]; ok && contractsIn != "" {
		err := json.Unmarshal([]byte(contractsIn), &contracts) // Expect a JSON array
		if err != nil {
			return args, fmt.Errorf("could not parse 'contracts': %w", err)
		}
	}

	// Initialize the event filter with the parsed arguments
	filter, err := state_stream.NewEventFilter(eventFilterConfig, chain, eventTypes.Flow(), addresses, contracts)
	if err != nil {
		return args, fmt.Errorf("failed to create event filter: %w", err)
	}
	args.Filter = filter

	return args, nil
}

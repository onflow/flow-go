package data_providers

import (
	"context"
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
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
)

type AccountStatusesArguments struct {
	StartBlockID     flow.Identifier                  // ID of the block to start subscription from
	StartBlockHeight uint64                           // Height of the block to start subscription from
	Filter           state_stream.AccountStatusFilter // Filter applied to events for a given subscription
}

type AccountStatusesDataProvider struct {
	*baseDataProvider

	logger         zerolog.Logger
	stateStreamApi state_stream.API
}

var _ DataProvider = (*AccountStatusesDataProvider)(nil)

// NewAccountStatusesDataProvider creates a new instance of AccountStatusesDataProvider.
func NewAccountStatusesDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	stateStreamApi state_stream.API,
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
	topic string,
	arguments models.Arguments,
	send chan<- interface{},
) (*AccountStatusesDataProvider, error) {
	p := &AccountStatusesDataProvider{
		logger:         logger.With().Str("component", "account-statuses-data-provider").Logger(),
		stateStreamApi: stateStreamApi,
	}

	accountStatusesArgs, err := ParseAccountStatusesArguments(arguments, chain, eventFilterConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for account statuses data provider: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	p.baseDataProvider = newBaseDataProvider(
		topic,
		cancel,
		send,
		p.createSubscription(subCtx, accountStatusesArgs), // Set up a subscription to events based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for events and handles responses.
//
// No errors are expected during normal operations.
func (p *AccountStatusesDataProvider) Run() error {
	return subscription.HandleSubscription(p.subscription, p.handleResponse(p.send))
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *AccountStatusesDataProvider) createSubscription(ctx context.Context, args AccountStatusesArguments) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.stateStreamApi.SubscribeAccountStatusesFromStartBlockID(ctx, args.StartBlockID, args.Filter)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.stateStreamApi.SubscribeAccountStatusesFromStartHeight(ctx, args.StartBlockHeight, args.Filter)
	}

	return p.stateStreamApi.SubscribeAccountStatusesFromLatestBlock(ctx, args.Filter)
}

// handleResponse processes an account statuses and sends the formatted response.
//
// No errors are expected during normal operations.
func (p *AccountStatusesDataProvider) handleResponse(send chan<- interface{}) func(accountStatusesResponse *backend.AccountStatusesResponse) error {
	messageIndex := counters.NewMonotonousCounter(0)

	return func(accountStatusesResponse *backend.AccountStatusesResponse) error {
		if ok := messageIndex.Set(messageIndex.Value() + 1); !ok {
			return status.Errorf(codes.Internal, "message index already incremented to %d", messageIndex.Value())
		}
		index := messageIndex.Value()

		send <- &models.AccountStatusesResponse{
			BlockID:       accountStatusesResponse.BlockID.String(),
			Height:        strconv.FormatUint(accountStatusesResponse.Height, 10),
			AccountEvents: accountStatusesResponse.AccountEvents,
			MessageIndex:  strconv.FormatUint(index, 10),
		}

		return nil
	}
}

// ParseAccountStatusesArguments validates and initializes the account statuses arguments.
func ParseAccountStatusesArguments(
	arguments models.Arguments,
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
) (AccountStatusesArguments, error) {
	var args AccountStatusesArguments

	// Check for mutual exclusivity of start_block_id and start_block_height early
	_, hasStartBlockID := arguments["start_block_id"]
	_, hasStartBlockHeight := arguments["start_block_height"]

	if hasStartBlockID && hasStartBlockHeight {
		return args, fmt.Errorf("can only provide either 'start_block_id' or 'start_block_height'")
	}

	// Parse 'start_block_id' if provided
	if hasStartBlockID {
		var startBlockID parser.ID
		err := startBlockID.Parse(arguments["start_block_id"])
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_id': %w", err)
		}
		args.StartBlockID = startBlockID.Flow()
	}

	// Parse 'start_block_height' if provided
	if hasStartBlockHeight {
		var err error
		args.StartBlockHeight, err = util.ToUint64(arguments["start_block_height"])
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_height': %w", err)
		}
	} else {
		args.StartBlockHeight = request.EmptyHeight
	}

	// Parse 'event_types' as []string{}
	var eventTypes parser.EventTypes
	if eventTypesIn, ok := arguments["event_types"]; ok && eventTypesIn != "" {
		err := eventTypes.Parse(strings.Split(eventTypesIn, ","))
		if err != nil {
			return args, fmt.Errorf("invalid 'event_types': %w", err)
		}
	}

	// Parse 'accountAddresses' as []string{}
	var accountAddresses []string
	if addressesIn, ok := arguments["accountAddresses"]; ok && addressesIn != "" {
		accountAddresses = strings.Split(addressesIn, ",")
	}

	// Initialize the event filter with the parsed arguments
	filter, err := state_stream.NewAccountStatusFilter(eventFilterConfig, chain, eventTypes.Flow(), accountAddresses)
	if err != nil {
		return args, fmt.Errorf("failed to create event filter: %w", err)
	}
	args.Filter = filter

	return args, nil
}

package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/common"
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

// accountStatusesArguments contains the arguments required for subscribing to account statuses
type accountStatusesArguments struct {
	StartBlockID      flow.Identifier                  // ID of the block to start subscription from
	StartBlockHeight  uint64                           // Height of the block to start subscription from
	Filter            state_stream.AccountStatusFilter // Filter applied to events for a given subscription
	HeartbeatInterval *uint64                          // Maximum number of blocks message won't be sent. Nil if not set
}

type AccountStatusesDataProvider struct {
	*baseDataProvider

	logger         zerolog.Logger
	stateStreamApi state_stream.API

	heartbeatInterval uint64
}

var _ DataProvider = (*AccountStatusesDataProvider)(nil)

// NewAccountStatusesDataProvider creates a new instance of AccountStatusesDataProvider.
func NewAccountStatusesDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	stateStreamApi state_stream.API,
	subscriptionID string,
	topic string,
	arguments models.Arguments,
	send chan<- interface{},
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
	heartbeatInterval uint64,
) (*AccountStatusesDataProvider, error) {
	if stateStreamApi == nil {
		return nil, fmt.Errorf("this access node does not support streaming account statuses")
	}

	p := &AccountStatusesDataProvider{
		logger:            logger.With().Str("component", "account-statuses-data-provider").Logger(),
		stateStreamApi:    stateStreamApi,
		heartbeatInterval: heartbeatInterval,
	}

	// Initialize arguments passed to the provider.
	accountStatusesArgs, err := parseAccountStatusesArguments(arguments, chain, eventFilterConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for account statuses data provider: %w", err)
	}
	if accountStatusesArgs.HeartbeatInterval != nil {
		p.heartbeatInterval = *accountStatusesArgs.HeartbeatInterval
	}

	subCtx, cancel := context.WithCancel(ctx)

	p.baseDataProvider = newBaseDataProvider(
		subscriptionID,
		topic,
		arguments,
		cancel,
		send,
		p.createSubscription(subCtx, accountStatusesArgs), // Set up a subscription to account statuses based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for events and handles responses.
//
// Expected errors during normal operations:
//   - context.Canceled: if the operation is canceled, during an unsubscribe action.
func (p *AccountStatusesDataProvider) Run() error {
	messageIndex := counters.NewMonotonicCounter(0)
	blocksSinceLastMessage := uint64(0)

	return run(
		p.closedChan,
		p.subscription,
		func(response *backend.AccountStatusesResponse) error {
			return p.sendResponse(response, &messageIndex, &blocksSinceLastMessage)
		},
	)
}

// sendResponse processes an account statuses message and sends it to data provider's channel.
//
// No errors are expected during normal operations.
func (p *AccountStatusesDataProvider) sendResponse(
	response *backend.AccountStatusesResponse,
	messageIndex *counters.StrictMonotonicCounter,
	blocksSinceLastMessage *uint64,
) error {
	// Only send a response if there's meaningful data to send
	// or the heartbeat interval limit is reached
	*blocksSinceLastMessage += 1
	accountEmittedEvents := len(response.AccountEvents) != 0
	reachedHeartbeatLimit := *blocksSinceLastMessage >= p.heartbeatInterval
	if !accountEmittedEvents && !reachedHeartbeatLimit {
		return nil
	}

	var accountStatusesPayload models.AccountStatusesResponse
	accountStatusesPayload.Build(response, messageIndex.Value())
	messageIndex.Increment()

	var resp models.BaseDataProvidersResponse
	resp.Build(p.ID(), p.Topic(), &accountStatusesPayload)

	p.send <- &resp
	*blocksSinceLastMessage = 0

	return nil
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *AccountStatusesDataProvider) createSubscription(ctx context.Context, args accountStatusesArguments) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.stateStreamApi.SubscribeAccountStatusesFromStartBlockID(ctx, args.StartBlockID, args.Filter)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.stateStreamApi.SubscribeAccountStatusesFromStartHeight(ctx, args.StartBlockHeight, args.Filter)
	}

	return p.stateStreamApi.SubscribeAccountStatusesFromLatestBlock(ctx, args.Filter)
}

// parseAccountStatusesArguments validates and initializes the account statuses arguments.
func parseAccountStatusesArguments(
	arguments models.Arguments,
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
) (accountStatusesArguments, error) {
	allowedFields := []string{
		"start_block_id",
		"start_block_height",
		"event_types",
		"account_addresses",
		"heartbeat_interval",
	}
	err := ensureAllowedFields(arguments, allowedFields)
	if err != nil {
		return accountStatusesArguments{}, err
	}

	var args accountStatusesArguments

	// Parse block arguments
	startBlockID, startBlockHeight, err := parseStartBlock(arguments)
	if err != nil {
		return args, err
	}
	args.StartBlockID = startBlockID
	args.StartBlockHeight = startBlockHeight

	// Parse 'event_types' as a JSON array
	var eventTypes parser.EventTypes
	if eventTypesIn, ok := arguments["event_types"]; ok && eventTypesIn != "" {
		result, err := common.ParseInterfaceToStrings(eventTypesIn)
		if err != nil {
			return accountStatusesArguments{}, fmt.Errorf("'event_types' must be an array of string")
		}

		err = eventTypes.Parse(result)
		if err != nil {
			return accountStatusesArguments{}, fmt.Errorf("invalid 'event_types': %w", err)
		}
	}

	// Parse 'accountAddresses' as []string{}
	var accountAddresses []string
	if accountAddressesIn, ok := arguments["account_addresses"]; ok && accountAddressesIn != "" {
		accountAddresses, err = common.ParseInterfaceToStrings(accountAddressesIn)
		if err != nil {
			return accountStatusesArguments{}, fmt.Errorf("'account_addresses' must be an array of string")
		}
	}

	var heartbeatInterval uint64
	if heartbeatIntervalIn, ok := arguments["heartbeat_interval"]; ok && heartbeatIntervalIn != "" {
		result, ok := heartbeatIntervalIn.(string)
		if !ok {
			return accountStatusesArguments{}, fmt.Errorf("'heartbeat_interval' must be a string")
		}

		heartbeatInterval, err = util.ToUint64(result)
		if err != nil {
			return accountStatusesArguments{}, fmt.Errorf("invalid 'heartbeat_interval': %w", err)
		}

		args.HeartbeatInterval = &heartbeatInterval
	}

	// Initialize the event filter with the parsed arguments
	args.Filter, err = state_stream.NewAccountStatusFilter(eventFilterConfig, chain, eventTypes.Flow(), accountAddresses)
	if err != nil {
		return accountStatusesArguments{}, fmt.Errorf("failed to create event filter: %w", err)
	}

	return args, nil
}

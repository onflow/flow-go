package data_providers

import (
	"context"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"
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

// accountStatusesArguments contains the arguments required for subscribing to account statuses
type accountStatusesArguments struct {
	StartBlockID        flow.Identifier                  // ID of the block to start subscription from
	StartBlockHeight    uint64                           // Height of the block to start subscription from
	Filter              state_stream.AccountStatusFilter // Filter applied to events for a given subscription
	HeartbeatInterval   uint64                           // Maximum number of blocks message won't be sent
	ExecutionStateQuery entities.ExecutionStateQuery
}

type AccountStatusesDataProvider struct {
	*baseDataProvider

	arguments              accountStatusesArguments
	messageIndex           counters.StrictMonotonicCounter
	blocksSinceLastMessage uint64
	stateStreamApi         state_stream.API
}

var _ DataProvider = (*AccountStatusesDataProvider)(nil)

// NewAccountStatusesDataProvider creates a new instance of AccountStatusesDataProvider.
func NewAccountStatusesDataProvider(
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
) (*AccountStatusesDataProvider, error) {
	if stateStreamApi == nil {
		return nil, fmt.Errorf("this access node does not support streaming account statuses")
	}

	args, err := parseAccountStatusesArguments(rawArguments, chain, eventFilterConfig, defaultHeartbeatInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for account statuses data provider: %w", err)
	}

	provider := newBaseDataProvider(
		ctx,
		logger.With().Str("component", "account-statuses-data-provider").Logger(),
		nil,
		subscriptionID,
		topic,
		rawArguments,
		send,
	)

	return &AccountStatusesDataProvider{
		baseDataProvider:       provider,
		arguments:              args,
		messageIndex:           counters.NewMonotonicCounter(0),
		blocksSinceLastMessage: 0,
		stateStreamApi:         stateStreamApi,
	}, nil
}

// Run starts processing the subscription for events and handles responses.
// Must be called once.
//
// No errors expected during normal operations.
func (p *AccountStatusesDataProvider) Run() error {
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
func (p *AccountStatusesDataProvider) handleResponse(response *backend.AccountStatusesResponse) error {
	// convert events to JSON-CDC format
	convertedResponse, err := convertAccountStatusesResponse(response)
	if err != nil {
		return fmt.Errorf("failed to convert account status events to JSON-CDC format: %w", err)
	}

	return p.sendResponse(convertedResponse)
}

// sendResponse processes an account statuses message and sends it to data provider's channel.
// This function is not safe to call concurrently.
//
// No errors are expected during normal operations
func (p *AccountStatusesDataProvider) sendResponse(response *backend.AccountStatusesResponse) error {
	// Only send a response if there's meaningful data to send
	// or the heartbeat interval limit is reached
	p.blocksSinceLastMessage += 1
	accountEmittedEvents := len(response.AccountEvents) != 0
	reachedHeartbeatLimit := p.blocksSinceLastMessage >= p.arguments.HeartbeatInterval
	if !accountEmittedEvents && !reachedHeartbeatLimit {
		return nil
	}

	accountStatusesPayload := models.NewAccountStatusesResponse(response, p.messageIndex.Value())
	resp := models.BaseDataProvidersResponse{
		SubscriptionID: p.ID(),
		Topic:          p.Topic(),
		Payload:        accountStatusesPayload,
	}
	p.send <- &resp

	p.blocksSinceLastMessage = 0
	p.messageIndex.Increment()

	return nil
}

// createAndStartSubscription creates a new subscription using the specified input arguments.
func (p *AccountStatusesDataProvider) createAndStartSubscription(
	ctx context.Context,
	args accountStatusesArguments,
) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.stateStreamApi.SubscribeAccountStatusesFromStartBlockID(ctx, args.StartBlockID, args.Filter, args.ExecutionStateQuery)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.stateStreamApi.SubscribeAccountStatusesFromStartHeight(ctx, args.StartBlockHeight, args.Filter, args.ExecutionStateQuery)
	}

	return p.stateStreamApi.SubscribeAccountStatusesFromLatestBlock(ctx, args.Filter, args.ExecutionStateQuery)
}

// convertAccountStatusesResponse converts events in the provided AccountStatusesResponse from CCF
// to JSON-CDC format.
//
// No errors expected during normal operations.
func convertAccountStatusesResponse(resp *backend.AccountStatusesResponse) (*backend.AccountStatusesResponse, error) {
	jsoncdcEvents := make(map[string]flow.EventsList, len(resp.AccountEvents))
	for eventType, events := range resp.AccountEvents {
		convertedEvents, err := convertEvents(events)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %s events to JSON-CDC: %w", eventType, err)
		}
		jsoncdcEvents[eventType] = convertedEvents
	}

	return &backend.AccountStatusesResponse{
		BlockID:       resp.BlockID,
		Height:        resp.Height,
		AccountEvents: jsoncdcEvents,
	}, nil
}

// parseAccountStatusesArguments validates and initializes the account statuses arguments.
func parseAccountStatusesArguments(
	arguments wsmodels.Arguments,
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
	defaultHeartbeatInterval uint64,
) (accountStatusesArguments, error) {
	allowedFields := map[string]struct{}{
		"start_block_id":        {},
		"start_block_height":    {},
		"event_types":           {},
		"account_addresses":     {},
		"heartbeat_interval":    {},
		"execution_state_query": {},
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

	// Parse 'heartbeat_interval' argument
	heartbeatInterval, err := extractHeartbeatInterval(arguments, defaultHeartbeatInterval)
	if err != nil {
		return accountStatusesArguments{}, err
	}
	args.HeartbeatInterval = heartbeatInterval

	// Parse 'event_types' as a JSON array
	eventTypes, err := extractArrayOfStrings(arguments, "event_types", false)
	if err != nil {
		return accountStatusesArguments{}, err
	}

	// Parse 'account_addresses' as []string
	accountAddresses, err := extractArrayOfStrings(arguments, "account_addresses", false)
	if err != nil {
		return accountStatusesArguments{}, err
	}

	// Initialize the event filter with the parsed arguments
	args.Filter, err = state_stream.NewAccountStatusFilter(eventFilterConfig, chain, eventTypes, accountAddresses)
	if err != nil {
		return accountStatusesArguments{}, fmt.Errorf("failed to create event filter: %w", err)
	}

	// Parse 'execution_state_query' as JSON object
	agreeingExecutorCount, requiredExecutorIDs, includeExecutorMetadata, err :=
		extractExecutionStateQueryFields(arguments, "execution_state_query", false)
	if err != nil {
		return accountStatusesArguments{}, fmt.Errorf("error extracting execution_state_query fields: %w", err)
	}
	args.ExecutionStateQuery = entities.ExecutionStateQuery{
		AgreeingExecutorsCount:  agreeingExecutorCount,
		RequiredExecutorId:      requiredExecutorIDs,
		IncludeExecutorMetadata: includeExecutorMetadata,
	}

	return args, nil
}

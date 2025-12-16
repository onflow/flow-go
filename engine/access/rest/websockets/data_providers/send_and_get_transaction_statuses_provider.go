package data_providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	commonparser "github.com/onflow/flow-go/engine/access/rest/common/parser"
	httpmodels "github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// sendAndGetTransactionStatusesArguments contains the arguments required for sending tx and subscribing to transaction statuses
type sendAndGetTransactionStatusesArguments struct {
	Transaction         flow.TransactionBody            // The transaction body to be sent and monitored.
	ExecutionStateQuery *httpmodels.ExecutionStateQuery `json:"execution_state_query"` // Optional execution state query for selecting execution results.
}

type SendAndGetTransactionStatusesDataProvider struct {
	*baseDataProvider

	arguments     sendAndGetTransactionStatusesArguments
	messageIndex  counters.StrictMonotonicCounter
	linkGenerator commonmodels.LinkGenerator
}

var _ DataProvider = (*SendAndGetTransactionStatusesDataProvider)(nil)

func NewSendAndGetTransactionStatusesDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	subscriptionID string,
	linkGenerator commonmodels.LinkGenerator,
	topic string,
	rawArguments wsmodels.Arguments,
	send chan<- interface{},
	chain flow.Chain,
) (*SendAndGetTransactionStatusesDataProvider, error) {
	args, err := parseSendAndGetTransactionStatusesArguments(rawArguments, chain)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for send tx statuses data provider: %w", err)
	}

	provider := newBaseDataProvider(
		ctx,
		logger.With().Str("component", "send-transaction-statuses-data-provider").Logger(),
		api,
		subscriptionID,
		topic,
		rawArguments,
		send,
	)

	return &SendAndGetTransactionStatusesDataProvider{
		baseDataProvider: provider,
		arguments:        args,
		messageIndex:     counters.NewMonotonicCounter(0),
		linkGenerator:    linkGenerator,
	}, nil
}

// Run starts processing the subscription for events and handles responses.
// Must be called once.
//
// No errors are expected during normal operations
func (p *SendAndGetTransactionStatusesDataProvider) Run() error {
	return run(
		p.createAndStartSubscription(p.ctx, p.arguments),
		p.sendResponse,
	)
}

// sendResponse processes a tx status message and sends it to client's channel.
// This function is not safe to call concurrently.
//
// No errors are expected during normal operations.
func (p *SendAndGetTransactionStatusesDataProvider) sendResponse(txResults []*accessmodel.TransactionResult) error {
	for i := range txResults {
		// TODO: Add executor metadata when available from subscription response
		txStatusesPayload := models.NewTransactionStatusesResponse(p.linkGenerator, txResults[i], nil, p.messageIndex.Value())
		response := models.BaseDataProvidersResponse{
			SubscriptionID: p.ID(),
			Topic:          p.Topic(),
			Payload:        txStatusesPayload,
		}
		p.send <- &response

		p.messageIndex.Increment()
	}

	return nil
}

// createAndStartSubscription creates a new subscription using the specified input arguments.
func (p *SendAndGetTransactionStatusesDataProvider) createAndStartSubscription(
	ctx context.Context,
	args sendAndGetTransactionStatusesArguments,
) subscription.Subscription {
	// Extract criteria from the execution state query if provided
	var criteria optimistic_sync.Criteria
	if args.ExecutionStateQuery != nil {
		criteria = httpmodels.NewCriteria(*args.ExecutionStateQuery)
	}
	return p.api.SendAndSubscribeTransactionStatuses(ctx, &args.Transaction, entities.EventEncodingVersion_JSON_CDC_V0, criteria)
}

// parseSendAndGetTransactionStatusesArguments validates and initializes the account statuses arguments.
func parseSendAndGetTransactionStatusesArguments(
	arguments wsmodels.Arguments,
	chain flow.Chain,
) (sendAndGetTransactionStatusesArguments, error) {
	var args sendAndGetTransactionStatusesArguments

	// Parse execution_state_query first if provided (optional)
	if rawQuery, exists := arguments["execution_state_query"]; exists && rawQuery != nil {
		query, err := parseExecutionStateQuery(rawQuery)
		if err != nil {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("invalid 'execution_state_query': %w", err)
		}
		args.ExecutionStateQuery = &query
	}

	// Extract transaction-specific fields, excluding execution_state_query
	// since it's not part of the transaction body
	txArgs := make(map[string]interface{})
	for key, value := range arguments {
		if key != "execution_state_query" {
			txArgs[key] = value
		}
	}

	// Convert the transaction arguments to JSON
	rawJSON, err := json.Marshal(txArgs)
	if err != nil {
		return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("failed to marshal arguments: %w", err)
	}

	// Create an io.Reader from the JSON bytes
	var tx commonparser.Transaction
	rawReader := bytes.NewReader(rawJSON)
	err = tx.Parse(rawReader, chain)
	if err != nil {
		return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("failed to parse transaction: %w", err)
	}

	args.Transaction = tx.Flow()

	return args, nil
}

// parseExecutionStateQuery parses the execution state query from raw WebSocket arguments.
func parseExecutionStateQuery(raw interface{}) (httpmodels.ExecutionStateQuery, error) {
	queryMap, ok := raw.(map[string]interface{})
	if !ok {
		return httpmodels.ExecutionStateQuery{}, fmt.Errorf("execution_state_query must be an object")
	}

	var query httpmodels.ExecutionStateQuery

	// Parse agreeing_executors_count (optional)
	if rawCount, exists := queryMap["agreeing_executors_count"]; exists && rawCount != nil {
		// Handle both float64 (JSON number) and string
		switch v := rawCount.(type) {
		case float64:
			query.AgreeingExecutorsCount = uint64(v)
		case string:
			// Try to parse as number if provided as string
			var count uint64
			_, err := fmt.Sscanf(v, "%d", &count)
			if err != nil {
				return httpmodels.ExecutionStateQuery{}, fmt.Errorf("invalid agreeing_executors_count: %w", err)
			}
			query.AgreeingExecutorsCount = count
		default:
			return httpmodels.ExecutionStateQuery{}, fmt.Errorf("agreeing_executors_count must be a number")
		}
	}

	// Parse required_executor_ids (optional)
	if rawIDs, exists := queryMap["required_executor_ids"]; exists && rawIDs != nil {
		idsArray, ok := rawIDs.([]interface{})
		if !ok {
			return httpmodels.ExecutionStateQuery{}, fmt.Errorf("required_executor_ids must be an array")
		}

		query.RequiredExecutorIDs = make([]flow.Identifier, 0, len(idsArray))
		for i, rawID := range idsArray {
			idString, ok := rawID.(string)
			if !ok {
				return httpmodels.ExecutionStateQuery{}, fmt.Errorf("required_executor_ids[%d] must be a string", i)
			}

			parsedID, err := commonparser.NewID(idString)
			if err != nil {
				return httpmodels.ExecutionStateQuery{}, fmt.Errorf("invalid required_executor_ids[%d]: %w", i, err)
			}

			query.RequiredExecutorIDs = append(query.RequiredExecutorIDs, parsedID.Flow())
		}
	}

	return query, nil
}

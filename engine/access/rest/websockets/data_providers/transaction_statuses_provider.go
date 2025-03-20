package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// transactionStatusesArguments contains the arguments required for subscribing to transaction statuses
type transactionStatusesArguments struct {
	TxID flow.Identifier `json:"tx_id"` // ID of the transaction to monitor.
}

// TransactionStatusesDataProvider is responsible for providing tx statuses
type TransactionStatusesDataProvider struct {
	*baseDataProvider

	logger        zerolog.Logger
	api           access.API
	linkGenerator commonmodels.LinkGenerator
}

var _ DataProvider = (*TransactionStatusesDataProvider)(nil)

func NewTransactionStatusesDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	subscriptionID string,
	linkGenerator commonmodels.LinkGenerator,
	topic string,
	arguments wsmodels.Arguments,
	send chan<- interface{},
) (*TransactionStatusesDataProvider, error) {
	p := &TransactionStatusesDataProvider{
		logger:        logger.With().Str("component", "transaction-statuses-data-provider").Logger(),
		api:           api,
		linkGenerator: linkGenerator,
	}

	// Initialize arguments passed to the provider.
	txStatusesArgs, err := parseTransactionStatusesArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for tx statuses data provider: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	p.baseDataProvider = newBaseDataProvider(
		subscriptionID,
		topic,
		arguments,
		cancel,
		send,
		p.createSubscription(subCtx, txStatusesArgs), // Set up a subscription to tx statuses based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for events and handles responses.
//
// Expected errors during normal operations:
//   - context.Canceled: if the operation is canceled, during an unsubscribe action.
func (p *TransactionStatusesDataProvider) Run() error {
	return subscription.HandleSubscription(p.subscription, p.handleResponse())
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *TransactionStatusesDataProvider) createSubscription(
	ctx context.Context,
	args transactionStatusesArguments,
) subscription.Subscription {
	return p.api.SubscribeTransactionStatuses(ctx, args.TxID, entities.EventEncodingVersion_JSON_CDC_V0)
}

// handleResponse processes a tx statuses and sends the formatted response.
//
// No errors are expected during normal operations.
func (p *TransactionStatusesDataProvider) handleResponse() func(txResults []*access.TransactionResult) error {
	messageIndex := counters.NewMonotonicCounter(0)

	return func(txResults []*access.TransactionResult) error {
		for i := range txResults {
			index := messageIndex.Value()
			if ok := messageIndex.Set(messageIndex.Value() + 1); !ok {
				return status.Errorf(codes.Internal, "message index already incremented to %d", messageIndex.Value())
			}

			txStatusesPayload := models.NewTransactionStatusesResponse(p.linkGenerator, txResults[i], index)
			response := models.BaseDataProvidersResponse{
				SubscriptionID: p.ID(),
				Topic:          p.Topic(),
				Payload:        txStatusesPayload,
			}
			p.send <- &response
		}

		return nil
	}
}

// parseAccountStatusesArguments validates and initializes the account statuses arguments.
func parseTransactionStatusesArguments(
	arguments wsmodels.Arguments,
) (transactionStatusesArguments, error) {
	allowedFields := map[string]struct{}{
		"tx_id": {},
	}
	err := ensureAllowedFields(arguments, allowedFields)
	if err != nil {
		return transactionStatusesArguments{}, err
	}

	var args transactionStatusesArguments

	// Check if tx_id exists and is not empty
	rawTxID, exists := arguments["tx_id"]
	if !exists {
		return transactionStatusesArguments{}, fmt.Errorf("missing 'tx_id' field")
	}

	// Ensure the transaction ID is a string
	txIDString, isString := rawTxID.(string)
	if !isString {
		return transactionStatusesArguments{}, fmt.Errorf("'tx_id' must be a string")
	}

	if len(txIDString) == 0 {
		return transactionStatusesArguments{}, fmt.Errorf("'tx_id' must not be empty")
	}

	var parsedTxID parser.ID
	if err = parsedTxID.Parse(txIDString); err != nil {
		return transactionStatusesArguments{}, fmt.Errorf("invalid 'tx_id': %w", err)
	}

	// Assign the validated transaction ID to the args
	args.TxID = parsedTxID.Flow()
	return args, nil
}

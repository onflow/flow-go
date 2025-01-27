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
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// transactionStatusesArguments contains the arguments required for subscribing to transaction statuses
type transactionStatusesArguments struct {
	TxID flow.Identifier // ID of the transaction to monitor.
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
	arguments models.Arguments,
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
// No errors are expected during normal operations.
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
	messageIndex := counters.NewMonotonousCounter(0)

	return func(txResults []*access.TransactionResult) error {

		for i := range txResults {
			index := messageIndex.Value()
			if ok := messageIndex.Set(messageIndex.Value() + 1); !ok {
				return status.Errorf(codes.Internal, "message index already incremented to %d", messageIndex.Value())
			}

			var txStatusesPayload models.TransactionStatusesResponse
			txStatusesPayload.Build(p.linkGenerator, txResults[i], index)

			var response models.BaseDataProvidersResponse
			response.Build(p.ID(), p.Topic(), &txStatusesPayload)

			p.send <- &response
		}

		return nil
	}
}

// parseAccountStatusesArguments validates and initializes the account statuses arguments.
func parseTransactionStatusesArguments(
	arguments models.Arguments,
) (transactionStatusesArguments, error) {
	var args transactionStatusesArguments

	if txIDIn, ok := arguments["tx_id"]; ok && txIDIn != "" {
		result, ok := txIDIn.(string)
		if !ok {
			return args, fmt.Errorf("'tx_id' must be a string")
		}
		var txID parser.ID
		err := txID.Parse(result)
		if err != nil {
			return args, fmt.Errorf("invalid 'tx_id': %w", err)
		}
		args.TxID = txID.Flow()
	}

	return args, nil
}

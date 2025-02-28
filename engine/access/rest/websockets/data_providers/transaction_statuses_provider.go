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
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// transactionStatusesArguments contains the arguments required for subscribing to transaction statuses
type transactionStatusesArguments struct {
	TxID             flow.Identifier // ID of the transaction to monitor.
	StartBlockID     flow.Identifier // ID of the block to start subscription from
	StartBlockHeight uint64          // Height of the block to start subscription from
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
	if args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeTransactionStatusesFromStartBlockID(ctx, args.TxID, args.StartBlockID, entities.EventEncodingVersion_JSON_CDC_V0)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeTransactionStatusesFromStartHeight(ctx, args.TxID, args.StartBlockHeight, entities.EventEncodingVersion_JSON_CDC_V0)
	}

	return p.api.SubscribeTransactionStatusesFromLatest(ctx, args.TxID, entities.EventEncodingVersion_JSON_CDC_V0)
}

// handleResponse processes a tx statuses and sends the formatted response.
//
// No errors are expected during normal operations.
func (p *TransactionStatusesDataProvider) handleResponse() func(txResults []*accessmodel.TransactionResult) error {
	messageIndex := counters.NewMonotonicCounter(0)

	return func(txResults []*accessmodel.TransactionResult) error {
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
	allowedFields := []string{
		"start_block_id",
		"start_block_height",
		"tx_id",
	}
	err := ensureAllowedFields(arguments, allowedFields)
	if err != nil {
		return transactionStatusesArguments{}, err
	}

	var args transactionStatusesArguments

	// Parse block arguments
	startBlockID, startBlockHeight, err := parseStartBlock(arguments)
	if err != nil {
		return transactionStatusesArguments{}, err
	}
	args.StartBlockID = startBlockID
	args.StartBlockHeight = startBlockHeight

	if txIDIn, ok := arguments["tx_id"]; ok && txIDIn != "" {
		result, ok := txIDIn.(string)
		if !ok {
			return transactionStatusesArguments{}, fmt.Errorf("'tx_id' must be a string")
		}
		var txID parser.ID
		err := txID.Parse(result)
		if err != nil {
			return transactionStatusesArguments{}, fmt.Errorf("invalid 'tx_id': %w", err)
		}
		args.TxID = txID.Flow()
	}

	return args, nil
}

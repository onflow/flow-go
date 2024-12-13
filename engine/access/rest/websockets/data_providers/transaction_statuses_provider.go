package data_providers

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"strconv"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
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

type TransactionStatusesDataProvider struct {
	*baseDataProvider

	logger zerolog.Logger
	api    access.API
}

var _ DataProvider = (*TransactionStatusesDataProvider)(nil)

func NewTransactionStatusesDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	topic string,
	arguments models.Arguments,
	send chan<- interface{},
) (*TransactionStatusesDataProvider, error) {
	p := &TransactionStatusesDataProvider{
		logger: logger.With().Str("component", "transaction-statuses-data-provider").Logger(),
		api:    api,
	}

	// Initialize arguments passed to the provider.
	txStatusesArgs, err := parseTransactionStatusesArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for tx statuses data provider: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	p.baseDataProvider = newBaseDataProvider(
		topic,
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

// handleResponse processes an account statuses and sends the formatted response.
//
// No errors are expected during normal operations.
func (p *TransactionStatusesDataProvider) handleResponse() func(txResults []*access.TransactionResult) error {
	messageIndex := counters.NewMonotonousCounter(1)

	return func(txResults []*access.TransactionResult) error {

		index := messageIndex.Value()
		if ok := messageIndex.Set(messageIndex.Value() + 1); !ok {
			return status.Errorf(codes.Internal, "message index already incremented to %d", messageIndex.Value())
		}

		p.send <- models.TransactionStatusesResponse{
			TransactionResults: txResults,
			MessageIndex:       strconv.FormatUint(index, 10),
		}

		return nil
	}
}

// parseAccountStatusesArguments validates and initializes the account statuses arguments.
func parseTransactionStatusesArguments(
	arguments models.Arguments,
) (transactionStatusesArguments, error) {
	var args transactionStatusesArguments

	// Check for mutual exclusivity of start_block_id and start_block_height early
	startBlockIDIn, hasStartBlockID := arguments["start_block_id"]
	startBlockHeightIn, hasStartBlockHeight := arguments["start_block_height"]

	if hasStartBlockID && hasStartBlockHeight {
		return args, fmt.Errorf("can only provide either 'start_block_id' or 'start_block_height'")
	}

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

	// Parse 'start_block_id' if provided
	if hasStartBlockID {
		result, ok := startBlockIDIn.(string)
		if !ok {
			return args, fmt.Errorf("'start_block_id' must be a string")
		}
		var startBlockID parser.ID
		err := startBlockID.Parse(result)
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_id': %w", err)
		}
		args.StartBlockID = startBlockID.Flow()
	}

	// Parse 'start_block_height' if provided
	var err error
	if hasStartBlockHeight {
		result, ok := startBlockHeightIn.(string)
		if !ok {
			return args, fmt.Errorf("'start_block_height' must be a string")
		}
		args.StartBlockHeight, err = util.ToUint64(result)
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_height': %w", err)
		}
	} else {
		args.StartBlockHeight = request.EmptyHeight
	}

	return args, nil
}

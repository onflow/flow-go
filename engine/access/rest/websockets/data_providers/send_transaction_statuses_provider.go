package data_providers

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
)

// sendTransactionStatusesArguments contains the arguments required for sending tx and subscribing to transaction statuses
type sendTransactionStatusesArguments struct {
	Transaction flow.TransactionBody // The transaction body to be sent and monitored.
}

type SendTransactionStatusesDataProvider struct {
	*baseDataProvider

	logger zerolog.Logger
	api    access.API
}

var _ DataProvider = (*SendTransactionStatusesDataProvider)(nil)

func NewSendTransactionStatusesDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	topic string,
	arguments models.Arguments,
	send chan<- interface{},
) (*SendTransactionStatusesDataProvider, error) {
	p := &SendTransactionStatusesDataProvider{
		logger: logger.With().Str("component", "send-transaction-statuses-data-provider").Logger(),
		api:    api,
	}

	// Initialize arguments passed to the provider.
	sendTxStatusesArgs, err := parseSendTransactionStatusesArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for send tx statuses data provider: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	p.baseDataProvider = newBaseDataProvider(
		topic,
		cancel,
		send,
		p.createSubscription(subCtx, sendTxStatusesArgs), // Set up a subscription to tx statuses based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for events and handles responses.
//
// No errors are expected during normal operations.
func (p *SendTransactionStatusesDataProvider) Run() error {
	return subscription.HandleSubscription(p.subscription, p.handleResponse())
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *SendTransactionStatusesDataProvider) createSubscription(
	ctx context.Context,
	args sendTransactionStatusesArguments,
) subscription.Subscription {
	return p.api.SendAndSubscribeTransactionStatuses(ctx, &args.Transaction, entities.EventEncodingVersion_JSON_CDC_V0)
}

// handleResponse processes an account statuses and sends the formatted response.
//
// No errors are expected during normal operations.
func (p *SendTransactionStatusesDataProvider) handleResponse() func(txResults []*access.TransactionResult) error {

	messageIndex := counters.NewMonotonousCounter(1)

	return func(txResults []*access.TransactionResult) error {

		index := messageIndex.Value()
		if ok := messageIndex.Set(messageIndex.Value() + 1); !ok {
			return status.Errorf(codes.Internal, "message index already incremented to %d", messageIndex.Value())
		}

		p.send <- &models.TransactionStatusesResponse{
			TransactionResults: txResults,
			MessageIndex:       strconv.FormatUint(index, 10),
		}

		return nil
	}

}

// parseAccountStatusesArguments validates and initializes the account statuses arguments.
func parseSendTransactionStatusesArguments(
	arguments models.Arguments,
) (sendTransactionStatusesArguments, error) {
	var args sendTransactionStatusesArguments
	var tx flow.TransactionBody

	if scriptIn, ok := arguments["script"]; ok && scriptIn != "" {
		script, ok := scriptIn.([]byte)
		if !ok {
			return args, fmt.Errorf("'script' must be a byte array")
		}

		tx.Script = script
	}

	if argumentsIn, ok := arguments["arguments"]; ok && argumentsIn != "" {
		argumentsData, ok := argumentsIn.([][]byte)
		if !ok {
			return args, fmt.Errorf("'arguments' must be a [][]byte type")
		}

		tx.Arguments = argumentsData
	}

	if referenceBlockIDIn, ok := arguments["reference_block_id"]; ok && referenceBlockIDIn != "" {
		result, ok := referenceBlockIDIn.(string)
		if !ok {
			return args, fmt.Errorf("'reference_block_id' must be a string")
		}

		var referenceBlockID parser.ID
		err := referenceBlockID.Parse(result)
		if err != nil {
			return args, fmt.Errorf("invalid 'reference_block_id': %w", err)
		}

		tx.ReferenceBlockID = referenceBlockID.Flow()
	}

	return args, nil
}

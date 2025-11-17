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
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// sendAndGetTransactionStatusesArguments contains the arguments required for sending a
// transaction and subscribing to its transaction status updates.
type sendAndGetTransactionStatusesArguments struct {
	Transaction flow.TransactionBody // The transaction body to be sent and monitored.
}

// SendAndGetTransactionStatusesDataProvider sends a transaction and streams its
// transaction status updates over a WebSocket subscription.
//
// Runtime:
//   - Use Run to start the subscription; it should be called once.
type SendAndGetTransactionStatusesDataProvider struct {
	*baseDataProvider

	arguments     sendAndGetTransactionStatusesArguments
	messageIndex  counters.StrictMonotonicCounter
	linkGenerator commonmodels.LinkGenerator
}

var _ DataProvider = (*SendAndGetTransactionStatusesDataProvider)(nil)

// NewSendAndGetTransactionStatusesDataProvider creates a new instance of
// SendAndGetTransactionStatusesDataProvider.
//
// Expected errors:
//   - [data_providers.ErrInvalidArgument]: The provided arguments are invalid.
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

// Run starts processing the subscription for transaction statuses and handles responses.
// Must be called once.
//
// No errors are expected during normal operations.
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
		txStatusesPayload := models.NewTransactionStatusesResponse(p.linkGenerator, txResults[i], p.messageIndex.Value())
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
	return p.api.SendAndSubscribeTransactionStatuses(ctx, &args.Transaction, entities.EventEncodingVersion_JSON_CDC_V0)
}

// parseSendAndGetTransactionStatusesArguments validates and initializes the arguments
// required to send a transaction and subscribe to its status updates.
//
// Input shape: a JSON object that serializes a Flow transaction body; the function uses
// the common transaction parser to validate and parse the transaction according to the
// provided chain.
//
// Expected errors:
//   - [data_providers.ErrInvalidArgument]: Not used by this function (listed for consistency).
func parseSendAndGetTransactionStatusesArguments(
	arguments wsmodels.Arguments,
	chain flow.Chain,
) (sendAndGetTransactionStatusesArguments, error) {
	var args sendAndGetTransactionStatusesArguments

	// Convert the arguments map to JSON
	rawJSON, err := json.Marshal(arguments)
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

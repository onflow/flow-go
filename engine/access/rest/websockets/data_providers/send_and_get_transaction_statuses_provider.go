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
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// sendAndGetTransactionStatusesArguments contains the arguments required for sending tx and subscribing to transaction statuses
type sendAndGetTransactionStatusesArguments struct {
	Transaction flow.TransactionBody // The transaction body to be sent and monitored.
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
	p.subscriptionState.subscription = p.createAndStartSubscription(p.ctx, p.arguments)

	return run(
		p.subscriptionState.subscription,
		func(response []*access.TransactionResult) error {
			return p.sendResponse(response)
		},
	)
}

// sendResponse processes a tx status message and sends it to client's channel.
// This function is not safe to call concurrently.
//
// No errors are expected during normal operations.
func (p *SendAndGetTransactionStatusesDataProvider) sendResponse(txResults []*access.TransactionResult) error {
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

// parseSendAndGetTransactionStatusesArguments validates and initializes the account statuses arguments.
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

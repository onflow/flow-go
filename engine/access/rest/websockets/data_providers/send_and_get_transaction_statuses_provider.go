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
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
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

	logger        zerolog.Logger
	api           access.API
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
	arguments models.Arguments,
	send chan<- interface{},
) (*SendAndGetTransactionStatusesDataProvider, error) {
	p := &SendAndGetTransactionStatusesDataProvider{
		logger:        logger.With().Str("component", "send-transaction-statuses-data-provider").Logger(),
		api:           api,
		linkGenerator: linkGenerator,
	}

	// Initialize arguments passed to the provider.
	sendTxStatusesArgs, err := parseSendAndGetTransactionStatusesArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for send tx statuses data provider: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	p.baseDataProvider = newBaseDataProvider(
		subscriptionID,
		topic,
		arguments,
		cancel,
		send,
		p.createSubscription(subCtx, sendTxStatusesArgs), // Set up a subscription to tx statuses based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for events and handles responses.
//
// No errors are expected during normal operations.
func (p *SendAndGetTransactionStatusesDataProvider) Run() error {
	return subscription.HandleSubscription(p.subscription, p.handleResponse())
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *SendAndGetTransactionStatusesDataProvider) createSubscription(
	ctx context.Context,
	args sendAndGetTransactionStatusesArguments,
) subscription.Subscription {
	return p.api.SendAndSubscribeTransactionStatuses(ctx, &args.Transaction, entities.EventEncodingVersion_JSON_CDC_V0)
}

// handleResponse processes a tx statuses and sends the formatted response.
//
// No errors are expected during normal operations.
func (p *SendAndGetTransactionStatusesDataProvider) handleResponse() func(txResults []*access.TransactionResult) error {
	messageIndex := counters.NewMonotonicCounter(0)

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

// parseSendAndGetTransactionStatusesArguments validates and initializes the account statuses arguments.
func parseSendAndGetTransactionStatusesArguments(
	arguments models.Arguments,
) (sendAndGetTransactionStatusesArguments, error) {
	allowedFields := []string{
		"reference_block_id",
		"script",
		"arguments",
		"gas_limit",
		"payer",
		"proposal_key",
		"authorizers",
		"payload_signatures",
		"envelope_signatures",
	}
	err := ensureAllowedFields(arguments, allowedFields)
	if err != nil {
		return sendAndGetTransactionStatusesArguments{}, err
	}

	var args sendAndGetTransactionStatusesArguments
	var tx flow.TransactionBody

	if scriptIn, ok := arguments["script"]; ok && scriptIn != "" {
		result, ok := scriptIn.(string)
		if !ok {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("'script' must be a string")
		}

		script, err := util.FromBase64(result)
		if err != nil {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("invalid 'script': %w", err)
		}

		tx.Script = script
	}

	if argumentsIn, ok := arguments["arguments"]; ok && argumentsIn != "" {
		result, ok := argumentsIn.([]string)
		if !ok {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("'arguments' must be a []string type")
		}

		var argumentsData [][]byte
		for _, arg := range result {
			argument, err := util.FromBase64(arg)
			if err != nil {
				return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("invalid 'arguments': %w", err)
			}

			argumentsData = append(argumentsData, argument)
		}

		tx.Arguments = argumentsData
	}

	if referenceBlockIDIn, ok := arguments["reference_block_id"]; ok && referenceBlockIDIn != "" {
		result, ok := referenceBlockIDIn.(string)
		if !ok {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("'reference_block_id' must be a string")
		}

		var referenceBlockID parser.ID
		err := referenceBlockID.Parse(result)
		if err != nil {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("invalid 'reference_block_id': %w", err)
		}

		tx.ReferenceBlockID = referenceBlockID.Flow()
	}

	if gasLimitIn, ok := arguments["gas_limit"]; ok && gasLimitIn != "" {
		result, ok := gasLimitIn.(string)
		if !ok {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("'gas_limit' must be a string")
		}

		gasLimit, err := util.ToUint64(result)
		if err != nil {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("invalid 'gas_limit': %w", err)
		}
		tx.GasLimit = gasLimit
	}

	if payerIn, ok := arguments["payer"]; ok && payerIn != "" {
		result, ok := payerIn.(string)
		if !ok {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("'payerIn' must be a string")
		}

		payerAddr, err := flow.StringToAddress(result)
		if err != nil {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("invalid 'payer': %w", err)
		}
		tx.Payer = payerAddr
	}

	if proposalKeyIn, ok := arguments["proposal_key"]; ok && proposalKeyIn != "" {
		proposalKey, ok := proposalKeyIn.(flow.ProposalKey)
		if !ok {
			return sendAndGetTransactionStatusesArguments{},
				fmt.Errorf("'proposal_key' must be a object (ProposalKey)")
		}

		tx.ProposalKey = proposalKey
	}

	if authorizersIn, ok := arguments["authorizers"]; ok && authorizersIn != "" {
		result, ok := authorizersIn.([]string)
		if !ok {
			return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("'authorizers' must be a []string type")
		}

		var authorizersData []flow.Address
		for _, auth := range result {
			authorizer, err := flow.StringToAddress(auth)
			if err != nil {
				return sendAndGetTransactionStatusesArguments{}, fmt.Errorf("invalid 'authorizers': %w", err)
			}

			authorizersData = append(authorizersData, authorizer)
		}

		tx.Authorizers = authorizersData
	}

	if payloadSignaturesIn, ok := arguments["payload_signatures"]; ok && payloadSignaturesIn != "" {
		payloadSignatures, ok := payloadSignaturesIn.([]flow.TransactionSignature)
		if !ok {
			return sendAndGetTransactionStatusesArguments{},
				fmt.Errorf("'payload_signatures' must be an array of objects (TransactionSignature)")
		}

		tx.PayloadSignatures = payloadSignatures
	}

	if envelopeSignaturesIn, ok := arguments["envelope_signatures"]; ok && envelopeSignaturesIn != "" {
		envelopeSignatures, ok := envelopeSignaturesIn.([]flow.TransactionSignature)
		if !ok {
			return sendAndGetTransactionStatusesArguments{},
				fmt.Errorf("'envelope_signatures' must be an array of objects (TransactionSignature)")
		}

		tx.EnvelopeSignatures = envelopeSignatures
	}
	args.Transaction = tx

	return args, nil
}

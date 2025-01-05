package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	commonparser "github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/rest/websockets/parser"
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
	messageIndex := counters.NewMonotonousCounter(0)

	return func(txResults []*access.TransactionResult) error {

		for i := range txResults {
			index := messageIndex.Value()
			if ok := messageIndex.Set(messageIndex.Value() + 1); !ok {
				return status.Errorf(codes.Internal, "message index already incremented to %d", messageIndex.Value())
			}

			var response models.TransactionStatusesResponse
			response.Build(p.linkGenerator, txResults[i], index)

			p.send <- &response
		}

		return nil
	}
}

// parseSendAndGetTransactionStatusesArguments validates and initializes the account statuses arguments.
func parseSendAndGetTransactionStatusesArguments(
	arguments models.Arguments,
) (sendAndGetTransactionStatusesArguments, error) {
	var args sendAndGetTransactionStatusesArguments
	var tx flow.TransactionBody

	if scriptIn, ok := arguments["script"]; ok && scriptIn != "" {
		result, ok := scriptIn.(string)
		if !ok {
			return args, fmt.Errorf("'script' must be a string")
		}

		script, err := util.FromBase64(result)
		if err != nil {
			return args, fmt.Errorf("invalid 'script': %w", err)
		}

		tx.Script = script
	}

	if argumentsIn, ok := arguments["arguments"]; ok && argumentsIn != "" {
		result, ok := argumentsIn.([]interface{})
		if !ok {
			return args, fmt.Errorf("'arguments' must be an array")
		}

		var argumentsData [][]byte
		for _, value := range result {
			arg, ok := value.(string)
			if !ok {
				return args, fmt.Errorf("'argument' must be a string")
			}

			argument, err := util.FromBase64(arg)
			if err != nil {
				return args, fmt.Errorf("invalid 'arguments': %w", err)
			}

			argumentsData = append(argumentsData, argument)
		}

		tx.Arguments = argumentsData
	}

	if referenceBlockIDIn, ok := arguments["reference_block_id"]; ok && referenceBlockIDIn != "" {
		result, ok := referenceBlockIDIn.(string)
		if !ok {
			return args, fmt.Errorf("'reference_block_id' must be a string")
		}

		var referenceBlockID commonparser.ID
		err := referenceBlockID.Parse(result)
		if err != nil {
			return args, fmt.Errorf("invalid 'reference_block_id': %w", err)
		}

		tx.ReferenceBlockID = referenceBlockID.Flow()
	}

	if gasLimitIn, ok := arguments["gas_limit"]; ok && gasLimitIn != "" {
		result, ok := gasLimitIn.(string)
		if !ok {
			return args, fmt.Errorf("'gas_limit' must be a string")
		}

		gasLimit, err := util.ToUint64(result)
		if err != nil {
			return args, fmt.Errorf("invalid 'gas_limit': %w", err)
		}
		tx.GasLimit = gasLimit
	}

	if payerIn, ok := arguments["payer"]; ok && payerIn != "" {
		result, ok := payerIn.(string)
		if !ok {
			return args, fmt.Errorf("'payer' must be a string")
		}

		payerAddr, err := flow.StringToAddress(result)
		if err != nil {
			return args, fmt.Errorf("invalid 'payer': %w", err)
		}
		tx.Payer = payerAddr
	}

	if proposalKeyIn, ok := arguments["proposal_key"]; ok && proposalKeyIn != "" {
		var proposalKey parser.ProposalKey
		err := proposalKey.Parse(proposalKeyIn)
		if err != nil {
			return args, fmt.Errorf("invalid 'proposal_key': %w", err)
		}

		tx.ProposalKey = proposalKey.Flow()
	}

	if authorizersIn, ok := arguments["authorizers"]; ok && authorizersIn != "" {
		result, ok := authorizersIn.([]interface{})
		if !ok {
			return args, fmt.Errorf("'authorizers' must be an array")
		}

		var authorizersData []flow.Address
		for i, auth := range result {
			authStr, ok := auth.(string)
			if !ok {
				return args, fmt.Errorf("invalid 'authorizer':%v, index: %d: must be a string", auth, i)
			}

			authorizer, err := flow.StringToAddress(authStr)
			if err != nil {
				return args, fmt.Errorf("invalid 'authorizers': %w", err)
			}

			authorizersData = append(authorizersData, authorizer)
		}

		tx.Authorizers = authorizersData
	}

	if payloadSignaturesIn, ok := arguments["payload_signatures"]; ok && payloadSignaturesIn != "" {
		signatures, ok := payloadSignaturesIn.([]interface{})
		if !ok {
			return args, fmt.Errorf("'payload_signatures' must be an array")
		}

		var payloadSignatures parser.TransactionSignatures
		err := payloadSignatures.Parse(signatures)
		if err != nil {
			return args, fmt.Errorf("invalid 'payload_signatures': %w", err)
		}

		tx.PayloadSignatures = payloadSignatures.Flow()
	}

	if envelopeSignaturesIn, ok := arguments["envelope_signatures"]; ok && envelopeSignaturesIn != "" {
		signatures, ok := envelopeSignaturesIn.([]interface{})
		if !ok {
			return args, fmt.Errorf("'envelope_signatures' must be an array")
		}

		var envelopeSignatures parser.TransactionSignatures
		err := envelopeSignatures.Parse(signatures)
		if err != nil {
			return args, fmt.Errorf("invalid 'envelope_signatures': %w", err)
		}

		tx.EnvelopeSignatures = envelopeSignatures.Flow()

	}
	args.Transaction = tx

	return args, nil
}

package data_providers

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
)

// BlockHeadersDataProvider is responsible for providing block headers
type BlockHeadersDataProvider struct {
	*baseDataProvider

	logger zerolog.Logger
	api    access.API
}

var _ DataProvider = (*BlockHeadersDataProvider)(nil)

// NewBlockHeadersDataProvider creates a new instance of BlockHeadersDataProvider.
func NewBlockHeadersDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	subscriptionID uuid.UUID,
	topic string,
	arguments models.Arguments,
	send chan<- interface{},
) (*BlockHeadersDataProvider, error) {
	p := &BlockHeadersDataProvider{
		logger: logger.With().Str("component", "block-headers-data-provider").Logger(),
		api:    api,
	}

	// Parse arguments passed to the provider.
	blockArgs, err := ParseBlocksArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	p.baseDataProvider = newBaseDataProvider(
		subscriptionID,
		topic,
		cancel,
		send,
		p.createSubscription(subCtx, blockArgs), // Set up a subscription to block headers based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for block headers and handles responses.
//
// No errors are expected during normal operations.
func (p *BlockHeadersDataProvider) Run() error {
	return subscription.HandleSubscription(
		p.subscription,
		subscription.HandleResponse(p.send, func(h *flow.Header) (interface{}, error) {
			var header commonmodels.BlockHeader
			header.Build(h)

			var response models.BaseDataProvidersResponse
			response.Build(
				p.ID().String(),
				p.Topic(),
				&header,
			)

			return &response, nil
		}),
	)
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *BlockHeadersDataProvider) createSubscription(ctx context.Context, args blocksArguments) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlockHeadersFromStartBlockID(ctx, args.StartBlockID, args.BlockStatus)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlockHeadersFromStartHeight(ctx, args.StartBlockHeight, args.BlockStatus)
	}

	return p.api.SubscribeBlockHeadersFromLatest(ctx, args.BlockStatus)
}

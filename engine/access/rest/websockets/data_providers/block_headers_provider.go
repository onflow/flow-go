package data_providers

import (
	"context"
	"fmt"

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

	arguments blocksArguments
}

var _ DataProvider = (*BlockHeadersDataProvider)(nil)

// NewBlockHeadersDataProvider creates a new instance of BlockHeadersDataProvider.
func NewBlockHeadersDataProvider(
	logger zerolog.Logger,
	api access.API,
	subscriptionID string,
	topic string,
	rawArguments models.Arguments,
	send chan<- interface{},
) (*BlockHeadersDataProvider, error) {
	args, err := parseBlocksArguments(rawArguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	base := newBaseDataProvider(
		logger.With().Str("component", "block-headers-data-provider").Logger(),
		api,
		subscriptionID,
		topic,
		rawArguments,
		send,
	)

	return &BlockHeadersDataProvider{
		baseDataProvider: base,
		arguments:        args,
	}, nil
}

// Run starts processing the subscription for block headers and handles responses.
//
// Expected errors during normal operations:
//   - context.Canceled: if the operation is canceled, during an unsubscribe action.
func (p *BlockHeadersDataProvider) Run(ctx context.Context) error {
	// we read data from the subscription and send them to client's channel
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p.subscriptionState = newSubscriptionState(cancel, p.createAndStartSubscription(ctx, p.arguments))

	return run(
		p.baseDataProvider.done,
		p.subscriptionState.subscription,
		func(h *flow.Header) error {
			var header commonmodels.BlockHeader
			header.Build(h)

			var response models.BaseDataProvidersResponse
			response.Build(p.ID(), p.Topic(), &header)
			p.send <- &response

			return nil
		},
	)
}

// createAndStartSubscription creates a new subscription using the specified input arguments.
func (p *BlockHeadersDataProvider) createAndStartSubscription(ctx context.Context, args blocksArguments) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlockHeadersFromStartBlockID(ctx, args.StartBlockID, args.BlockStatus)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlockHeadersFromStartHeight(ctx, args.StartBlockHeight, args.BlockStatus)
	}

	return p.api.SubscribeBlockHeadersFromLatest(ctx, args.BlockStatus)
}

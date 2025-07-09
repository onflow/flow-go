package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
)

// BlockDigestsDataProvider is responsible for providing block digests
type BlockDigestsDataProvider struct {
	*baseDataProvider

	arguments blocksArguments
}

var _ DataProvider = (*BlockDigestsDataProvider)(nil)

// NewBlockDigestsDataProvider creates a new instance of BlockDigestsDataProvider.
func NewBlockDigestsDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	subscriptionID string,
	topic string,
	rawArguments wsmodels.Arguments,
	send chan<- interface{},
) (*BlockDigestsDataProvider, error) {
	args, err := parseBlocksArguments(rawArguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	base := newBaseDataProvider(
		ctx,
		logger.With().Str("component", "block-digests-data-provider").Logger(),
		api,
		subscriptionID,
		topic,
		rawArguments,
		send,
	)

	return &BlockDigestsDataProvider{
		baseDataProvider: base,
		arguments:        args,
	}, nil
}

// Run starts processing the subscription for block digests and handles responses.
// Must be called once.
//
// No errors expected during normal operations
func (p *BlockDigestsDataProvider) Run() error {
	return run(
		p.createAndStartSubscription(p.ctx, p.arguments),
		p.sendResponse,
	)
}

// createAndStartSubscription creates a new subscription using the specified input arguments.
func (p *BlockDigestsDataProvider) createAndStartSubscription(
	ctx context.Context,
	args blocksArguments,
) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlockDigestsFromStartBlockID(ctx, args.StartBlockID, args.BlockStatus)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlockDigestsFromStartHeight(ctx, args.StartBlockHeight, args.BlockStatus)
	}

	return p.api.SubscribeBlockDigestsFromLatest(ctx, args.BlockStatus)
}

func (p *BlockDigestsDataProvider) sendResponse(b *flow.BlockDigest) error {
	blockDigest := models.NewBlockDigest(b)
	response := models.BaseDataProvidersResponse{
		SubscriptionID: p.ID(),
		Topic:          p.Topic(),
		Payload:        blockDigest,
	}
	p.send <- &response

	return nil
}

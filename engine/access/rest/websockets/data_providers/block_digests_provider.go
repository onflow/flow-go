package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
)

// BlockDigestsDataProvider is responsible for providing block digests
type BlockDigestsDataProvider struct {
	*BaseDataProviderImpl

	logger zerolog.Logger
	args   BlocksArguments
	api    access.API
}

var _ DataProvider = (*BlockDigestsDataProvider)(nil)

// NewBlockDigestsDataProvider creates a new instance of BlockDigestsDataProvider.
func NewBlockDigestsDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	topic string,
	arguments map[string]string,
	send chan<- interface{},
) (*BlockDigestsDataProvider, error) {
	p := &BlockDigestsDataProvider{
		logger: logger.With().Str("component", "block-digests-data-provider").Logger(),
		api:    api,
	}

	// Initialize arguments passed to the provider.
	var err error
	p.args, err = ParseBlocksArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	context, cancel := context.WithCancel(ctx)

	p.BaseDataProviderImpl = NewBaseDataProviderImpl(
		topic,
		cancel,
		send,
		p.createSubscription(context), // Set up a subscription to block digests based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for block digests and handles responses.
//
// No errors are expected during normal operations.
func (p *BlockDigestsDataProvider) Run() error {
	return subscription.HandleSubscription(p.subscription, p.handleResponse(p.send))
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *BlockDigestsDataProvider) createSubscription(ctx context.Context) subscription.Subscription {
	if p.args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlockDigestsFromStartBlockID(ctx, p.args.StartBlockID, p.args.BlockStatus)
	}

	if p.args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlockDigestsFromStartHeight(ctx, p.args.StartBlockHeight, p.args.BlockStatus)
	}

	return p.api.SubscribeBlockDigestsFromLatest(ctx, p.args.BlockStatus)
}

// handleResponse processes a block digest and sends the formatted response.
//
// No errors are expected during normal operations.
func (p *BlockDigestsDataProvider) handleResponse(send chan<- interface{}) func(block *flow.BlockDigest) error {
	return func(block *flow.BlockDigest) error {
		send <- &models.BlockDigestMessageResponse{
			Block: block,
		}

		return nil
	}
}

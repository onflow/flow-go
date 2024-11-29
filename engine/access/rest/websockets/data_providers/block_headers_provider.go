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
	return subscription.HandleSubscription(p.subscription, p.handleResponse(p.send))
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *BlockHeadersDataProvider) createSubscription(ctx context.Context, args BlocksArguments) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlockHeadersFromStartBlockID(ctx, args.StartBlockID, args.BlockStatus)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlockHeadersFromStartHeight(ctx, args.StartBlockHeight, args.BlockStatus)
	}

	return p.api.SubscribeBlockHeadersFromLatest(ctx, args.BlockStatus)
}

// handleResponse processes a block header and sends the formatted response.
//
// No errors are expected during normal operations.
func (p *BlockHeadersDataProvider) handleResponse(send chan<- interface{}) func(header *flow.Header) error {
	return func(header *flow.Header) error {
		send <- &models.BlockHeaderMessageResponse{
			Header: header,
		}

		return nil
	}
}

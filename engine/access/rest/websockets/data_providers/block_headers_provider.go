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
	*BaseDataProviderImpl

	logger zerolog.Logger
	args   BlocksArguments
	api    access.API
}

var _ DataProvider = (*BlockHeadersDataProvider)(nil)

// NewBlockHeadersDataProvider creates a new instance of BlockHeadersDataProvider.
func NewBlockHeadersDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	topic string,
	arguments map[string]string,
	send chan<- interface{},
) (*BlockHeadersDataProvider, error) {
	p := &BlockHeadersDataProvider{
		logger: logger.With().Str("component", "block-headers-data-provider").Logger(),
		api:    api,
	}

	// Initialize arguments passed to the provider.
	var err error
	p.args, err = ParseBlocksArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	p.BaseDataProviderImpl = NewBaseDataProviderImpl(
		topic,
		cancel,
		send,
		p.createSubscription(ctx), // Set up a subscription to block headers based on arguments.
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
func (p *BlockHeadersDataProvider) createSubscription(ctx context.Context) subscription.Subscription {
	if p.args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlockHeadersFromStartBlockID(ctx, p.args.StartBlockID, p.args.BlockStatus)
	}

	if p.args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlockHeadersFromStartHeight(ctx, p.args.StartBlockHeight, p.args.BlockStatus)
	}

	return p.api.SubscribeBlockHeadersFromLatest(ctx, p.args.BlockStatus)
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

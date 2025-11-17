package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
)

// BlockHeadersDataProvider streams block headers over a WebSocket subscription.
//
// Runtime:
//   - Use Run to start the subscription; it should be called once.
type BlockHeadersDataProvider struct {
	*baseDataProvider

	arguments blocksArguments
}

var _ DataProvider = (*BlockHeadersDataProvider)(nil)

// NewBlockHeadersDataProvider creates a new instance of BlockHeadersDataProvider.
//
// Expected errors:
//   - [data_providers.ErrInvalidArgument]: The provided subscription arguments are invalid.
func NewBlockHeadersDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	subscriptionID string,
	topic string,
	rawArguments wsmodels.Arguments,
	send chan<- interface{},
) (*BlockHeadersDataProvider, error) {
	args, err := parseBlocksArguments(rawArguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	base := newBaseDataProvider(
		ctx,
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
// Must be called once.
//
// No errors are expected during normal operations.
func (p *BlockHeadersDataProvider) Run() error {
	return run(
		p.createAndStartSubscription(p.ctx, p.arguments),
		p.sendResponse,
	)
}

// createAndStartSubscription creates a new subscription using the specified input arguments.
func (p *BlockHeadersDataProvider) createAndStartSubscription(
	ctx context.Context,
	args blocksArguments,
) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlockHeadersFromStartBlockID(ctx, args.StartBlockID, args.BlockStatus)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlockHeadersFromStartHeight(ctx, args.StartBlockHeight, args.BlockStatus)
	}

	return p.api.SubscribeBlockHeadersFromLatest(ctx, args.BlockStatus)
}

func (p *BlockHeadersDataProvider) sendResponse(header *flow.Header) error {
	headerPayload := commonmodels.NewBlockHeader(header)
	response := models.BaseDataProvidersResponse{
		SubscriptionID: p.ID(),
		Topic:          p.Topic(),
		Payload:        headerPayload,
	}
	p.send <- &response

	return nil
}

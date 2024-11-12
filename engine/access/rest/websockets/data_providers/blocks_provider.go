package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
)

// BlocksArguments contains the arguments required for subscribing to blocks
type BlocksArguments struct {
	StartBlockID     flow.Identifier  // ID of the block to start subscription from
	StartBlockHeight uint64           // Height of the block to start subscription from
	BlockStatus      flow.BlockStatus // Status of blocks to subscribe to
}

// BlocksDataProvider is responsible for providing blocks
type BlocksDataProvider struct {
	*BaseDataProviderImpl

	logger zerolog.Logger
	args   BlocksArguments
	api    access.API
}

var _ DataProvider = (*BlocksDataProvider)(nil)

// NewBlocksDataProvider creates a new instance of BlocksDataProvider.
func NewBlocksDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	topic string,
	arguments map[string]string,
	send chan<- interface{},
) (*BlocksDataProvider, error) {
	p := &BlocksDataProvider{
		logger: logger.With().Str("component", "block-data-provider").Logger(),
		api:    api,
	}

	// Validate arguments passed to the provider.
	err := p.validateArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	context, cancel := context.WithCancel(ctx)

	// Set up a subscription to blocks based on the provided start block ID and block status.
	subscription := p.createSubscription(context)
	p.BaseDataProviderImpl = NewBaseDataProviderImpl(
		context,
		cancel,
		topic,
		send,
		subscription,
	)

	return p, nil
}

// Run starts processing the subscription for blocks and handles responses.
//
// No errors are expected during normal operations.
func (p *BlocksDataProvider) Run() error {
	return subscription.HandleSubscription(p.ctx, p.subscription, handleResponse(p.send, p.args.BlockStatus))
}

// validateArguments checks and validates the arguments passed to the provider.
//
// No errors are expected during normal operations.
func (p *BlocksDataProvider) validateArguments(arguments map[string]string) error {
	// Parse 'block_status'
	if blockStatusIn, ok := arguments["block_status"]; ok {
		blockStatus, err := parser.ParseBlockStatus(blockStatusIn)
		if err != nil {
			return err
		}
		p.args.BlockStatus = blockStatus
	} else {
		return fmt.Errorf("'block_status' must be provided")
	}

	// Parse 'start_block_id' if provided
	if startBlockIDIn, ok := arguments["start_block_id"]; ok {
		var startBlockID parser.ID
		err := startBlockID.Parse(startBlockIDIn)
		if err != nil {
			return err
		}
		p.args.StartBlockID = startBlockID.Flow()
	}

	// Parse 'start_block_height' if provided
	if startBlockHeightIn, ok := arguments["start_block_height"]; ok {
		var err error
		p.args.StartBlockHeight, err = util.ToUint64(startBlockHeightIn)
		if err != nil {
			return fmt.Errorf("invalid 'start_block_height': %w", err)
		}
	} else {
		p.args.StartBlockHeight = request.EmptyHeight
	}

	// if both start_block_id and start_height are provided
	if p.args.StartBlockID != flow.ZeroID && p.args.StartBlockHeight != request.EmptyHeight {
		return fmt.Errorf("can only provide either 'start_block_id' or 'start_block_height'")
	}

	return nil
}

// createSubscription creates a new subscription based on the specified start block ID or height.
func (p *BlocksDataProvider) createSubscription(ctx context.Context) subscription.Subscription {
	if p.args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlocksFromStartBlockID(ctx, p.args.StartBlockID, p.args.BlockStatus)
	}

	if p.args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlocksFromStartHeight(ctx, p.args.StartBlockHeight, p.args.BlockStatus)
	}

	return p.api.SubscribeBlocksFromLatest(ctx, p.args.BlockStatus)
}

// handleResponse processes a block and sends the formatted response.
//
// No errors are expected during normal operations.
func handleResponse(send chan<- interface{}, blockStatus flow.BlockStatus) func(*flow.Block) error {
	return func(block *flow.Block) error {
		send <- &models.BlockMessageResponse{
			Block:       block,
			BlockStatus: blockStatus,
		}

		return nil
	}
}

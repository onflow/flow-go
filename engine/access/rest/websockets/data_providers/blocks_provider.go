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

// BlocksArguments contains the arguments required for subscribing to blocks / block headers / block digests
type BlocksArguments struct {
	StartBlockID     flow.Identifier  // ID of the block to start subscription from
	StartBlockHeight uint64           // Height of the block to start subscription from
	BlockStatus      flow.BlockStatus // Status of blocks to subscribe to
}

// BlocksDataProvider is responsible for providing blocks
type BlocksDataProvider struct {
	*baseDataProvider

	logger zerolog.Logger
	api    access.API
}

var _ DataProvider = (*BlocksDataProvider)(nil)

// NewBlocksDataProvider creates a new instance of BlocksDataProvider.
func NewBlocksDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	topic string,
	arguments models.Arguments,
	send chan<- interface{},
) (*BlocksDataProvider, error) {
	p := &BlocksDataProvider{
		logger: logger.With().Str("component", "blocks-data-provider").Logger(),
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
		p.createSubscription(subCtx, blockArgs), // Set up a subscription to blocks based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for blocks and handles responses.
//
// No errors are expected during normal operations.
func (p *BlocksDataProvider) Run() error {
	return subscription.HandleSubscription(
		p.subscription,
		subscription.HandleResponse(p.send, func(block *flow.Block) (interface{}, error) {
			return &models.BlockMessageResponse{
				Block: block,
			}, nil
		}),
	)
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *BlocksDataProvider) createSubscription(ctx context.Context, args BlocksArguments) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlocksFromStartBlockID(ctx, args.StartBlockID, args.BlockStatus)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlocksFromStartHeight(ctx, args.StartBlockHeight, args.BlockStatus)
	}

	return p.api.SubscribeBlocksFromLatest(ctx, args.BlockStatus)
}

// ParseBlocksArguments validates and initializes the blocks arguments.
func ParseBlocksArguments(arguments models.Arguments) (BlocksArguments, error) {
	var args BlocksArguments

	// Parse 'block_status'
	if blockStatusIn, ok := arguments["block_status"]; ok {
		blockStatus, err := parser.ParseBlockStatus(blockStatusIn)
		if err != nil {
			return args, err
		}
		args.BlockStatus = blockStatus
	} else {
		return args, fmt.Errorf("'block_status' must be provided")
	}

	startBlockIDIn, hasStartBlockID := arguments["start_block_id"]
	startBlockHeightIn, hasStartBlockHeight := arguments["start_block_height"]

	// Ensure only one of start_block_id or start_block_height is provided
	if hasStartBlockID && hasStartBlockHeight {
		return args, fmt.Errorf("can only provide either 'start_block_id' or 'start_block_height'")
	}

	// Parse 'start_block_id' if provided
	if hasStartBlockID {
		var startBlockID parser.ID
		err := startBlockID.Parse(startBlockIDIn)
		if err != nil {
			return args, err
		}
		args.StartBlockID = startBlockID.Flow()
	}

	// Parse 'start_block_height' if provided
	if hasStartBlockHeight {
		var err error
		args.StartBlockHeight, err = util.ToUint64(startBlockHeightIn)
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_height': %w", err)
		}
	} else {
		args.StartBlockHeight = request.EmptyHeight
	}

	return args, nil
}

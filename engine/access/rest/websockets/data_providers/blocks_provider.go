package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
)

// BlocksArguments contains the arguments required for subscribing to blocks / block headers / block digests
type blocksArguments struct {
	StartBlockID     flow.Identifier  // ID of the block to start subscription from
	StartBlockHeight uint64           // Height of the block to start subscription from
	BlockStatus      flow.BlockStatus // Status of blocks to subscribe to
}

// BlocksDataProvider is responsible for providing blocks
type BlocksDataProvider struct {
	*baseDataProvider

	logger        zerolog.Logger
	api           access.API
	arguments     blocksArguments
	linkGenerator commonmodels.LinkGenerator
}

var _ DataProvider = (*BlocksDataProvider)(nil)

// NewBlocksDataProvider creates a new instance of BlocksDataProvider.
func NewBlocksDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	subscriptionID string,
	linkGenerator commonmodels.LinkGenerator,
	topic string,
	arguments models.Arguments,
	send chan<- interface{},
) (*BlocksDataProvider, error) {
	p := &BlocksDataProvider{
		logger:        logger.With().Str("component", "blocks-data-provider").Logger(),
		api:           api,
		linkGenerator: linkGenerator,
	}

	// Parse arguments passed to the provider.
	var err error
	p.arguments, err = parseBlocksArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	p.baseDataProvider = newBaseDataProvider(
		subscriptionID,
		topic,
		arguments,
		cancel,
		send,
		p.createSubscription(subCtx, p.arguments), // Set up a subscription to blocks based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for blocks and handles responses.
//
// No errors are expected during normal operations.
func (p *BlocksDataProvider) Run() error {
	return subscription.HandleSubscription(
		p.subscription,
		subscription.HandleResponse(p.send, func(b *flow.Block) (interface{}, error) {
			var block commonmodels.Block

			expandPayload := map[string]bool{commonmodels.ExpandableFieldPayload: true}
			err := block.Build(b, nil, p.linkGenerator, p.arguments.BlockStatus, expandPayload)
			if err != nil {
				return nil, fmt.Errorf("failed to build block response :%w", err)
			}

			var response models.BaseDataProvidersResponse
			response.Build(p.ID(), p.Topic(), &block)

			return &response, nil
		}),
	)
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *BlocksDataProvider) createSubscription(ctx context.Context, args blocksArguments) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlocksFromStartBlockID(ctx, args.StartBlockID, args.BlockStatus)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlocksFromStartHeight(ctx, args.StartBlockHeight, args.BlockStatus)
	}

	return p.api.SubscribeBlocksFromLatest(ctx, args.BlockStatus)
}

// parseBlocksArguments validates and initializes the blocks arguments.
func parseBlocksArguments(arguments models.Arguments) (blocksArguments, error) {
	allowedFields := []string{
		"start_block_id",
		"start_block_height",
		"block_status",
	}
	err := ensureAllowedFields(arguments, allowedFields)
	if err != nil {
		return blocksArguments{}, err
	}

	var args blocksArguments

	// Parse 'block_status'
	if blockStatusIn, ok := arguments["block_status"]; ok {
		result, ok := blockStatusIn.(string)
		if !ok {
			return blocksArguments{}, fmt.Errorf("'block_status' must be string")
		}
		blockStatus, err := parser.ParseBlockStatus(result)
		if err != nil {
			return blocksArguments{}, err
		}
		args.BlockStatus = blockStatus
	} else {
		return blocksArguments{}, fmt.Errorf("'block_status' must be provided")
	}

	// Parse block arguments
	startBlockID, startBlockHeight, err := parseStartBlock(arguments)
	if err != nil {
		return blocksArguments{}, err
	}
	args.StartBlockID = startBlockID
	args.StartBlockHeight = startBlockHeight

	return args, nil
}

func parseStartBlock(arguments models.Arguments) (flow.Identifier, uint64, error) {
	startBlockIDIn, hasStartBlockID := arguments["start_block_id"]
	startBlockHeightIn, hasStartBlockHeight := arguments["start_block_height"]

	// Check for mutual exclusivity of start_block_id and start_block_height early
	if hasStartBlockID && hasStartBlockHeight {
		return flow.ZeroID, 0, fmt.Errorf("can only provide either 'start_block_id' or 'start_block_height'")
	}

	// Parse 'start_block_id'
	if hasStartBlockID {
		result, ok := startBlockIDIn.(string)
		if !ok {
			return flow.ZeroID, request.EmptyHeight, fmt.Errorf("'start_block_id' must be a string")
		}
		var startBlockID parser.ID
		err := startBlockID.Parse(result)
		if err != nil {
			return flow.ZeroID, request.EmptyHeight, fmt.Errorf("invalid 'start_block_id': %w", err)
		}
		return startBlockID.Flow(), request.EmptyHeight, nil
	}

	// Parse 'start_block_height'
	if hasStartBlockHeight {
		result, ok := startBlockHeightIn.(string)
		if !ok {
			return flow.ZeroID, 0, fmt.Errorf("'start_block_height' must be a string")
		}
		startBlockHeight, err := util.ToUint64(result)
		if err != nil {
			return flow.ZeroID, request.EmptyHeight, fmt.Errorf("invalid 'start_block_height': %w", err)
		}
		return flow.ZeroID, startBlockHeight, nil
	}

	return flow.ZeroID, request.EmptyHeight, nil
}

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
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
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
	rawArguments wsmodels.Arguments,
	send chan<- interface{},
) (*BlocksDataProvider, error) {
	args, err := parseBlocksArguments(rawArguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	provider := newBaseDataProvider(
		ctx,
		logger.With().Str("component", "blocks-data-provider").Logger(),
		api,
		subscriptionID,
		topic,
		rawArguments,
		send,
	)

	return &BlocksDataProvider{
		baseDataProvider: provider,
		arguments:        args,
		linkGenerator:    linkGenerator,
	}, nil
}

// Run starts processing the subscription for blocks and handles responses.
// Must be called once.
//
// No errors expected during normal operations
func (p *BlocksDataProvider) Run() error {
	return run(
		p.createAndStartSubscription(p.ctx, p.arguments),
		func(block *flow.Block) error {
			expandPayload := map[string]bool{commonmodels.ExpandableFieldPayload: true}
			blockPayload, err := commonmodels.NewBlock(
				block,
				nil,
				p.linkGenerator,
				p.arguments.BlockStatus,
				expandPayload,
			)
			if err != nil {
				return fmt.Errorf("failed to build block payload response: %w", err)
			}

			response := models.BaseDataProvidersResponse{
				SubscriptionID: p.ID(),
				Topic:          p.Topic(),
				Payload:        blockPayload,
			}
			p.send <- &response

			return nil
		},
	)
}

// createAndStartSubscription creates a new subscription using the specified input arguments.
func (p *BlocksDataProvider) createAndStartSubscription(
	ctx context.Context,
	args blocksArguments,
) subscription.Subscription {
	if args.StartBlockID != flow.ZeroID {
		return p.api.SubscribeBlocksFromStartBlockID(ctx, args.StartBlockID, args.BlockStatus)
	}

	if args.StartBlockHeight != request.EmptyHeight {
		return p.api.SubscribeBlocksFromStartHeight(ctx, args.StartBlockHeight, args.BlockStatus)
	}

	return p.api.SubscribeBlocksFromLatest(ctx, args.BlockStatus)
}

// parseBlocksArguments validates and initializes the blocks arguments.
func parseBlocksArguments(arguments wsmodels.Arguments) (blocksArguments, error) {
	allowedFields := map[string]struct{}{
		"start_block_id":     {},
		"start_block_height": {},
		"block_status":       {},
	}
	err := ensureAllowedFields(arguments, allowedFields)
	if err != nil {
		return blocksArguments{}, err
	}

	var args blocksArguments

	// Parse block arguments
	startBlockID, startBlockHeight, err := parseStartBlock(arguments)
	if err != nil {
		return blocksArguments{}, err
	}
	args.StartBlockID = startBlockID
	args.StartBlockHeight = startBlockHeight

	// Parse 'block_status'
	rawBlockStatus, exists := arguments["block_status"]
	if !exists {
		return blocksArguments{}, fmt.Errorf("missing 'block_status' field")
	}

	blockStatusStr, isString := rawBlockStatus.(string)
	if !isString {
		return blocksArguments{}, fmt.Errorf("'block_status' must be string")
	}

	if len(blockStatusStr) == 0 {
		return blocksArguments{}, fmt.Errorf("'block_status' field must not be empty")
	}

	blockStatus, err := parser.ParseBlockStatus(blockStatusStr)
	if err != nil {
		return blocksArguments{}, err
	}
	args.BlockStatus = blockStatus

	return args, nil
}

func parseStartBlock(arguments wsmodels.Arguments) (flow.Identifier, uint64, error) {
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
			return flow.ZeroID, request.EmptyHeight, fmt.Errorf("'start_block_height' must be convertible to uint64: %w", err)
		}
		return flow.ZeroID, startBlockHeight, nil
	}

	return flow.ZeroID, request.EmptyHeight, nil
}

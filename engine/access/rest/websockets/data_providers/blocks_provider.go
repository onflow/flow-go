package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
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
	Expand           map[string]bool
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
	p.arguments, err = ParseBlocksArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	p.baseDataProvider = newBaseDataProvider(
		topic,
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

			executionResult, err := p.getExecutionResult(b)
			if err != nil {
				return nil, err
			}

			err = block.Build(b, executionResult, p.linkGenerator, p.arguments.BlockStatus, p.arguments.Expand)
			if err != nil {
				return nil, fmt.Errorf("failed to build block response :%w", err)
			}

			return &models.BlockMessageResponse{
				Block: &block,
			}, nil
		}),
	)
}

// getExecutionResult retrieves the execution result for the given block.
// If the execution result is not yet available, it returns a nil execution result and no error.
//
// No errors are expected during normal operations.
func (p *BlocksDataProvider) getExecutionResult(b *flow.Block) (*flow.ExecutionResult, error) {
	executionResult, err := p.api.GetExecutionResultForBlockID(context.TODO(), b.ID())
	if err != nil {
		if se, ok := status.FromError(err); ok && se.Code() == codes.NotFound {
			return nil, nil // Execution result not yet available
		}
		return nil, fmt.Errorf("failed to get execution result for block: %s, %d: %w", b.ID(), b.Header.Height, err)
	}
	return executionResult, nil
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

// ParseBlocksArguments validates and initializes the blocks arguments.
func ParseBlocksArguments(arguments models.Arguments) (blocksArguments, error) {
	var args blocksArguments

	// Parse 'block_status'
	if blockStatusIn, ok := arguments["block_status"]; ok {
		result, ok := blockStatusIn.(string)
		if !ok {
			return args, fmt.Errorf("'block_status' must be string")
		}
		blockStatus, err := parser.ParseBlockStatus(result)
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

	if hasStartBlockID {
		result, ok := startBlockIDIn.(string)
		if !ok {
			return args, fmt.Errorf("'start_block_id' must be a string")
		}
		var startBlockID parser.ID
		err := startBlockID.Parse(result)
		if err != nil {
			return args, err
		}
		args.StartBlockID = startBlockID.Flow()
	}

	if hasStartBlockHeight {
		result, ok := startBlockHeightIn.(string)
		if !ok {
			return args, fmt.Errorf("'start_block_height' must be a string")
		}
		var err error
		args.StartBlockHeight, err = util.ToUint64(result)
		if err != nil {
			return args, fmt.Errorf("invalid 'start_block_height': %w", err)
		}
	} else {
		// Default value if 'start_block_height' is not provided
		args.StartBlockHeight = request.EmptyHeight
	}

	// Parse 'expand' as a JSON array of string
	// expected values: "payload", "execution_result"
	if expandIn, ok := arguments["expand"]; ok && expandIn != "" {
		result, ok := expandIn.([]string)
		if !ok {
			return args, fmt.Errorf("'expand' must be an array of string")
		}

		args.Expand = common.SliceToMap(result)
	}

	return args, nil
}

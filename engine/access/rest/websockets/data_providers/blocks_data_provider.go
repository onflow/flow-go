package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common/convert"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/model/flow"
)

// BlocksFromStartBlockIDArgs contains the arguments required for subscribing to blocks
// starting from a specific block ID.
type BlocksFromStartBlockIDArgs struct {
	StartBlockID flow.Identifier
	BlockStatus  flow.BlockStatus
}

// BlocksFromStartBlockIDProvider is responsible for providing blocks starting
// from a specific block ID.
type BlocksFromStartBlockIDProvider struct {
	*BaseDataProviderImpl

	logger zerolog.Logger
	args   BlocksFromStartBlockIDArgs
}

var _ DataProvider = (*BlocksFromStartBlockIDProvider)(nil)

// NewBlocksFromStartBlockIDProvider creates a new instance of BlocksFromStartBlockIDProvider.
func NewBlocksFromStartBlockIDProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	topic string,
	arguments map[string]string,
	send chan<- interface{},
) (*BlocksFromStartBlockIDProvider, error) {
	ctx, cancel := context.WithCancel(ctx)

	p := &BlocksFromStartBlockIDProvider{
		logger: logger.With().Str("component", "block-from-start-block-id-provider").Logger(),
	}

	// Validate arguments passed to the provider.
	err := p.validateArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Subscribe to blocks from the start block ID with the specified block status.
	subscription := p.api.SubscribeBlocksFromStartBlockID(ctx, p.args.StartBlockID, p.args.BlockStatus)
	p.BaseDataProviderImpl = NewBaseDataProviderImpl(
		ctx,
		cancel,
		api,
		topic,
		send,
		subscription,
	)

	return p, nil
}

// Run starts processing the subscription for blocks from the start block ID and handles responses.
//
// No errors are expected during normal operations.
func (p *BlocksFromStartBlockIDProvider) Run() error {
	return HandleSubscription(p.ctx, p.subscription, handleResponse(p.send, p.args.BlockStatus))
}

// validateArguments checks and validates the arguments passed to the provider.
//
// No errors are expected during normal operations.
func (p *BlocksFromStartBlockIDProvider) validateArguments(arguments map[string]string) error {
	// Check for block_status argument and validate
	if blockStatusIn, ok := arguments["block_status"]; ok {
		blockStatus, err := convert.ParseBlockStatus(blockStatusIn)
		if err != nil {
			return err
		}
		p.args.BlockStatus = blockStatus
	} else {
		return fmt.Errorf("'block_status' must be provided")
	}

	// Check for start_block_id argument and validate
	if startBlockIDIn, ok := arguments["start_block_id"]; ok {
		var startBlockID convert.ID
		err := startBlockID.Parse(startBlockIDIn)
		if err != nil {
			return err
		}
		p.args.StartBlockID = startBlockID.Flow()
	} else {
		return fmt.Errorf("'start_block_id' must be provided")
	}

	return nil
}

// BlocksFromBlockHeightArgs contains the arguments required for subscribing to blocks
// starting from a specific block height.
type BlocksFromBlockHeightArgs struct {
	StartBlockHeight uint64
	BlockStatus      flow.BlockStatus
}

// BlocksFromStartBlockHeightProvider is responsible for providing blocks starting
// from a specific block height.
type BlocksFromStartBlockHeightProvider struct {
	*BaseDataProviderImpl

	logger zerolog.Logger
	args   BlocksFromBlockHeightArgs
}

var _ DataProvider = (*BlocksFromStartBlockHeightProvider)(nil)

// NewBlocksFromStartBlockHeightProvider creates a new instance of BlocksFromStartBlockHeightProvider.
func NewBlocksFromStartBlockHeightProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	topic string,
	arguments map[string]string,
	send chan<- interface{},
) (*BlocksFromStartBlockHeightProvider, error) {
	ctx, cancel := context.WithCancel(ctx)

	p := &BlocksFromStartBlockHeightProvider{
		logger: logger.With().Str("component", "block-from-start-block-height-provider").Logger(),
	}

	// Validate arguments passed to the provider.
	err := p.validateArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Subscribe to blocks from the start block height with the specified block status.
	subscription := p.api.SubscribeBlocksFromStartHeight(ctx, p.args.StartBlockHeight, p.args.BlockStatus)
	p.BaseDataProviderImpl = NewBaseDataProviderImpl(
		ctx,
		cancel,
		api,
		topic,
		send,
		subscription,
	)

	return p, nil
}

// Run starts processing the subscription for blocks from the start block height and handles responses.
//
// No errors are expected during normal operations.
func (p *BlocksFromStartBlockHeightProvider) Run() error {
	return HandleSubscription(p.ctx, p.subscription, handleResponse(p.send, p.args.BlockStatus))
}

// validateArguments checks and validates the arguments passed to the provider.
//
// No errors are expected during normal operations.
func (p *BlocksFromStartBlockHeightProvider) validateArguments(arguments map[string]string) error {
	// Check for block_status argument and validate
	if blockStatusIn, ok := arguments["block_status"]; ok {
		blockStatus, err := convert.ParseBlockStatus(blockStatusIn)
		if err != nil {
			return err
		}
		p.args.BlockStatus = blockStatus
	} else {
		return fmt.Errorf("'block_status' must be provided")
	}

	if startBlockHeightIn, ok := arguments["start_block_height"]; ok {
		var err error
		p.args.StartBlockHeight, err = util.ToUint64(startBlockHeightIn)
		if err != nil {
			return fmt.Errorf("invalid start height: %w", err)
		}
	} else {
		return fmt.Errorf("'start_block_height' must be provided")
	}

	return nil
}

// BlocksFromLatestArgs contains the arguments required for subscribing to blocks
// starting from latest block.
type BlocksFromLatestArgs struct {
	BlockStatus flow.BlockStatus
}

// BlocksFromLatestProvider is responsible for providing blocks starting from latest block.
type BlocksFromLatestProvider struct {
	*BaseDataProviderImpl

	logger zerolog.Logger
	args   BlocksFromLatestArgs
}

var _ DataProvider = (*BlocksFromLatestProvider)(nil)

// NewBlocksFromLatestProvider creates a new BlocksFromLatestProvider.
func NewBlocksFromLatestProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	topic string,
	arguments map[string]string,
	send chan<- interface{},
) (*BlocksFromLatestProvider, error) {
	ctx, cancel := context.WithCancel(ctx)

	p := &BlocksFromLatestProvider{
		logger: logger.With().Str("component", "block-from-latest-provider").Logger(),
	}

	// Validate arguments passed to the provider.
	err := p.validateArguments(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Subscribe to blocks from the latest block height with the specified block status.
	subscription := p.api.SubscribeBlocksFromLatest(ctx, p.args.BlockStatus)
	p.BaseDataProviderImpl = NewBaseDataProviderImpl(
		ctx,
		cancel,
		api,
		topic,
		send,
		subscription,
	)

	return p, nil
}

// Run starts processing the subscription for blocks from the latest block and handles responses.
//
// No errors are expected during normal operations.
func (p *BlocksFromLatestProvider) Run() error {
	return HandleSubscription(p.ctx, p.subscription, handleResponse(p.send, p.args.BlockStatus))
}

// validateArguments checks and validates the arguments passed to the provider.
//
// No errors are expected during normal operations.
func (p *BlocksFromLatestProvider) validateArguments(arguments map[string]string) error {
	// Check for block_status argument and validate
	if blockStatusIn, ok := arguments["block_status"]; ok {
		blockStatus, err := convert.ParseBlockStatus(blockStatusIn)
		if err != nil {
			return err
		}
		p.args.BlockStatus = blockStatus
	} else {
		return fmt.Errorf("'block_status' must be provided")
	}

	return nil
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

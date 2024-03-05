package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// backendSubscribeBlocks is a struct representing a backend implementation for subscribing to blocks.
type backendSubscribeBlocks struct {
	log            zerolog.Logger
	state          protocol.State
	blocks         storage.Blocks
	headers        storage.Headers
	broadcaster    *engine.Broadcaster
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	blockTracker subscription.BlockTracker
}

// SubscribeBlocks subscribes to the finalized or sealed blocks starting at the requested
// start block, up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Each block is filtered by the provided block status, and only
// those blocks that match the status are returned.
//
// Only one of startBlockID or startHeight may be provided, Otherwise, an InvalidArgument error is returned.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlocks will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlocks(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribe(ctx, startBlockID, startHeight, b.getBlockResponse(blockStatus))
}

// SubscribeBlockHeaders streams finalized or sealed block headers starting at the requested
// start block, up until the latest available block header. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block header as it becomes available.
//
// Each block header are filtered by the provided block status, and only
// those block headers that match the status are returned.
//
// Only one of startBlockID or startHeight may be provided, Otherwise, an InvalidArgument error is returned.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlockHeaders will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlockHeaders(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribe(ctx, startBlockID, startHeight, b.getBlockHeaderResponse(blockStatus))
}

// SubscribeBlockDigests streams finalized or sealed lightweight block starting at the requested
// start block, up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Each lightweight block are filtered by the provided block status, and only
// those blocks that match the status are returned.
//
// Only one of startBlockID or startHeight may be provided, Otherwise, an InvalidArgument error is returned.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlockDigests will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlockDigests(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribe(ctx, startBlockID, startHeight, b.getBlockDigestResponse(blockStatus))
}

// subscribe is common method that allows clients to subscribe to different types of data.
// Only one of startBlockID or startHeight may be provided, Otherwise, subscribe will return a failed subscription.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
//
// Parameters:
// - ctx: The context for the subscription.
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startBlockHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
// - getData: The callback used by subscriptions to retrieve data information for the specified height and block status.
//
// If invalid parameters are supplied, subscribe will return a failed subscription.
func (b *backendSubscribeBlocks) subscribe(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, getData subscription.GetDataByHeightFunc) subscription.Subscription {
	nextHeight, err := b.blockTracker.GetStartHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	sub := subscription.NewHeightBasedSubscription(b.sendBufferSize, nextHeight, getData)
	go subscription.NewStreamer(b.log, b.broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

// getBlockResponse returns a GetDataByHeightFunc that retrieves block information for the specified height.
func (b *backendSubscribeBlocks) getBlockResponse(blockStatus flow.BlockStatus) subscription.GetDataByHeightFunc {
	return func(_ context.Context, height uint64) (interface{}, error) {
		block, err := b.getBlock(height, blockStatus)
		if err != nil {
			return nil, err
		}

		b.log.Trace().
			Hex("block_id", logging.ID(block.ID())).
			Uint64("height", height).
			Msgf("sending block info")

		return block, nil
	}
}

// getBlockHeaderResponse returns a GetDataByHeightFunc that retrieves block header information for the specified height.
func (b *backendSubscribeBlocks) getBlockHeaderResponse(blockStatus flow.BlockStatus) subscription.GetDataByHeightFunc {
	return func(_ context.Context, height uint64) (interface{}, error) {
		header, err := b.getBlockHeader(height, blockStatus)
		if err != nil {
			return nil, err
		}

		b.log.Trace().
			Hex("block_id", logging.ID(header.ID())).
			Uint64("height", height).
			Msgf("sending block header info")

		return header, nil
	}
}

// getBlockDigestResponse returns a GetDataByHeightFunc that retrieves lightweight block information for the specified height.
func (b *backendSubscribeBlocks) getBlockDigestResponse(blockStatus flow.BlockStatus) subscription.GetDataByHeightFunc {
	return func(_ context.Context, height uint64) (interface{}, error) {
		header, err := b.getBlockHeader(height, blockStatus)
		if err != nil {
			return nil, err
		}

		b.log.Trace().
			Hex("block_id", logging.ID(header.ID())).
			Uint64("height", height).
			Msgf("sending lightweight block info")

		return &flow.BlockDigest{
			ID:        header.ID(),
			Height:    header.Height,
			Timestamp: header.Timestamp,
		}, nil
	}
}

// getBlockHeader returns the block header for the given block height.
// Expected errors during normal operation:
// - storage.ErrNotFound: block for the given block height is not available.
func (b *backendSubscribeBlocks) getBlockHeader(height uint64, expectedBlockStatus flow.BlockStatus) (*flow.Header, error) {
	err := b.validateHeight(height, expectedBlockStatus)
	if err != nil {
		return nil, err
	}

	// since we are querying a finalized or sealed block header, we can use the height index and save an ID computation
	header, err := b.headers.ByHeight(height)
	if err != nil {
		return nil, err
	}

	return header, nil
}

// getBlock returns the block for the given block height.
// Expected errors during normal operation:
// - storage.ErrNotFound: block for the given block height is not available.
func (b *backendSubscribeBlocks) getBlock(height uint64, expectedBlockStatus flow.BlockStatus) (*flow.Block, error) {
	err := b.validateHeight(height, expectedBlockStatus)
	if err != nil {
		return nil, err
	}

	// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// validateHeight checks if the given block height is valid and available based on the expected block status.
// Expected errors during normal operation:
// - storage.ErrNotFound: block for the given block height is not available.
func (b *backendSubscribeBlocks) validateHeight(height uint64, expectedBlockStatus flow.BlockStatus) error {
	highestHeight, err := b.blockTracker.GetHighestHeight(expectedBlockStatus)
	if err != nil {
		return fmt.Errorf("could not get highest available height: %w", err)
	}

	// fail early if no notification has been received for the given block height.
	// note: it's possible for the data to exist in the data store before the notification is
	// received. this ensures a consistent view is available to all streams.
	if height > highestHeight {
		return fmt.Errorf("block %d is not available yet: %w", height, storage.ErrNotFound)
	}

	return nil
}

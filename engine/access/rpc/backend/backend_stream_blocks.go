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

// SubscribeBlocksFromStartBlockID subscribes to the finalized or sealed blocks starting at the requested
// start block id, up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Each block is filtered by the provided block status, and only
// those blocks that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlocksFromStartBlockID will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlocksFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribeFromStartBlockID(ctx, startBlockID, b.getBlockResponse(blockStatus))
}

// SubscribeBlocksFromStartHeight subscribes to the finalized or sealed blocks starting at the requested
// start block height, up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Each block is filtered by the provided block status, and only
// those blocks that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startHeight: The height of the starting block.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlocksFromStartHeight will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlocksFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribeFromStartHeight(ctx, startHeight, b.getBlockResponse(blockStatus))
}

// SubscribeBlocksFromLatest subscribes to the finalized or sealed blocks starting at the latest sealed block,
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Each block is filtered by the provided block status, and only
// those blocks that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlocksFromLatest will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlocksFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribeFromLatest(ctx, b.getBlockResponse(blockStatus))
}

// SubscribeBlockHeadersFromStartBlockID streams finalized or sealed block headers starting at the requested
// start block id, up until the latest available block header. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block header as it becomes available.
//
// Each block header are filtered by the provided block status, and only
// those block headers that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlockHeadersFromStartBlockID will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlockHeadersFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribeFromStartBlockID(ctx, startBlockID, b.getBlockHeaderResponse(blockStatus))
}

// SubscribeBlockHeadersFromStartHeight streams finalized or sealed block headers starting at the requested
// start block height, up until the latest available block header. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block header as it becomes available.
//
// Each block header are filtered by the provided block status, and only
// those block headers that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startHeight: The height of the starting block.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlockHeadersFromStartHeight will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlockHeadersFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribeFromStartHeight(ctx, startHeight, b.getBlockHeaderResponse(blockStatus))
}

// SubscribeBlockHeadersFromLatest streams finalized or sealed block headers starting at the latest sealed block,
// up until the latest available block header. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block header as it becomes available.
//
// Each block header are filtered by the provided block status, and only
// those block headers that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlockHeadersFromLatest will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlockHeadersFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribeFromLatest(ctx, b.getBlockHeaderResponse(blockStatus))
}

// SubscribeBlockDigestsFromStartBlockID streams finalized or sealed lightweight block starting at the requested
// start block id, up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Each lightweight block are filtered by the provided block status, and only
// those blocks that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlockDigestsFromStartBlockID will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlockDigestsFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribeFromStartBlockID(ctx, startBlockID, b.getBlockDigestResponse(blockStatus))
}

// SubscribeBlockDigestsFromStartHeight streams finalized or sealed lightweight block starting at the requested
// start block height, up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Each lightweight block are filtered by the provided block status, and only
// those blocks that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startHeight: The height of the starting block.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlockDigestsFromStartHeight will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlockDigestsFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribeFromStartHeight(ctx, startHeight, b.getBlockDigestResponse(blockStatus))
}

// SubscribeBlockDigestsFromLatest streams finalized or sealed lightweight block starting at the latest sealed block,
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Each lightweight block are filtered by the provided block status, and only
// those blocks that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters will be supplied SubscribeBlockDigestsFromLatest will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlockDigestsFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription {
	return b.subscribeFromLatest(ctx, b.getBlockDigestResponse(blockStatus))
}

// subscribeFromStartBlockID is common method that allows clients to subscribe starting at the requested start block id.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block.
// - getData: The callback used by subscriptions to retrieve data information for the specified height and block status.
//
// If invalid parameters are supplied, subscribeFromStartBlockID will return a failed subscription.
func (b *backendSubscribeBlocks) subscribeFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, getData subscription.GetDataByHeightFunc) subscription.Subscription {
	nextHeight, err := b.blockTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block id")
	}
	return b.subscribe(ctx, nextHeight, getData)
}

// subscribeFromStartHeight is common method that allows clients to subscribe starting at the requested start block height.
//
// Parameters:
// - ctx: Context for the operation.
// - startHeight: The height of the starting block.
// - getData: The callback used by subscriptions to retrieve data information for the specified height and block status.
//
// If invalid parameters are supplied, subscribeFromStartHeight will return a failed subscription.
func (b *backendSubscribeBlocks) subscribeFromStartHeight(ctx context.Context, startHeight uint64, getData subscription.GetDataByHeightFunc) subscription.Subscription {
	nextHeight, err := b.blockTracker.GetStartHeightFromHeight(startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}
	return b.subscribe(ctx, nextHeight, getData)
}

// subscribeFromLatest is common method that allows clients to subscribe starting at the latest sealed block.
//
// Parameters:
// - ctx: Context for the operation.
// - getData: The callback used by subscriptions to retrieve data information for the specified height and block status.
//
// No errors are expected during normal operation.
func (b *backendSubscribeBlocks) subscribeFromLatest(ctx context.Context, getData subscription.GetDataByHeightFunc) subscription.Subscription {
	nextHeight, err := b.blockTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from latest")
	}
	return b.subscribe(ctx, nextHeight, getData)
}

// subscribe is common method that allows clients to subscribe to different types of data.
//
// Parameters:
// - ctx: The context for the subscription.
// - nextHeight: The height of the starting block.
// - getData: The callback used by subscriptions to retrieve data information for the specified height and block status.
//
// No errors are expected during normal operation.
func (b *backendSubscribeBlocks) subscribe(ctx context.Context, nextHeight uint64, getData subscription.GetDataByHeightFunc) subscription.Subscription {
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

		return flow.NewBlockDigest(header.ID(), header.Height, header.Timestamp), nil
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
// - subscription.ErrBlockNotReady when unable to retrieve the block by height
func (b *backendSubscribeBlocks) validateHeight(height uint64, expectedBlockStatus flow.BlockStatus) error {
	highestHeight, err := b.blockTracker.GetHighestHeight(expectedBlockStatus)
	if err != nil {
		return fmt.Errorf("could not get highest available height: %w", err)
	}

	// fail early if no notification has been received for the given block height.
	// note: it's possible for the data to exist in the data store before the notification is
	// received. this ensures a consistent view is available to all streams.
	if height > highestHeight {
		return fmt.Errorf("block %d is not available yet: %w", height, subscription.ErrBlockNotReady)
	}

	return nil
}

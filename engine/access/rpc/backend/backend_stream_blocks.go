package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/height_source"
	"github.com/onflow/flow-go/engine/access/subscription/streamer"
	subimpl "github.com/onflow/flow-go/engine/access/subscription/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// backendSubscribeBlocks is a struct representing a backend implementation for subscribing to blocks.
type backendSubscribeBlocks struct {
	log     zerolog.Logger
	state   protocol.State
	blocks  storage.Blocks
	headers storage.Headers

	blockTracker              subscription.BlockTracker
	finalizedBlockBroadcaster *engine.Broadcaster
	streamOptions             *streamer.StreamOptions
	endHeight                 uint64
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
func (b *backendSubscribeBlocks) SubscribeBlocksFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	blockStatus flow.BlockStatus,
) subscription.Subscription[*flow.Block] {
	startHeight, err := b.blockTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subimpl.NewFailedSubscription[*flow.Block](err, "could not get start height from latest")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.buildReadyUpToHeight(blockStatus),
		b.blockAtHeight,
	)

	sub := subimpl.NewSubscription[*flow.Block](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.finalizedBlockBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
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
func (b *backendSubscribeBlocks) SubscribeBlocksFromStartHeight(
	ctx context.Context,
	startHeight uint64,
	blockStatus flow.BlockStatus,
) subscription.Subscription[*flow.Block] {
	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.buildReadyUpToHeight(blockStatus),
		b.blockAtHeight,
	)

	sub := subimpl.NewSubscription[*flow.Block](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.finalizedBlockBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
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
func (b *backendSubscribeBlocks) SubscribeBlocksFromLatest(
	ctx context.Context,
	blockStatus flow.BlockStatus,
) subscription.Subscription[*flow.Block] {
	startHeight, err := b.blockTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subimpl.NewFailedSubscription[*flow.Block](err, "could not get start height from latest")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.buildReadyUpToHeight(blockStatus),
		b.blockAtHeight,
	)

	sub := subimpl.NewSubscription[*flow.Block](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.finalizedBlockBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
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
func (b *backendSubscribeBlocks) SubscribeBlockHeadersFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	blockStatus flow.BlockStatus,
) subscription.Subscription[*flow.Header] {
	startHeight, err := b.blockTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subimpl.NewFailedSubscription[*flow.Header](err, "could not get start height from latest")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.buildReadyUpToHeight(blockStatus),
		b.headerAtHeight,
	)

	sub := subimpl.NewSubscription[*flow.Header](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.finalizedBlockBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
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
func (b *backendSubscribeBlocks) SubscribeBlockHeadersFromStartHeight(
	ctx context.Context,
	startHeight uint64,
	blockStatus flow.BlockStatus,
) subscription.Subscription[*flow.Header] {
	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.buildReadyUpToHeight(blockStatus),
		b.headerAtHeight,
	)

	sub := subimpl.NewSubscription[*flow.Header](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.finalizedBlockBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
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
func (b *backendSubscribeBlocks) SubscribeBlockHeadersFromLatest(
	ctx context.Context,
	blockStatus flow.BlockStatus,
) subscription.Subscription[*flow.Header] {
	startHeight, err := b.blockTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subimpl.NewFailedSubscription[*flow.Header](err, "could not get start height from latest")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.buildReadyUpToHeight(blockStatus),
		b.headerAtHeight,
	)

	sub := subimpl.NewSubscription[*flow.Header](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.finalizedBlockBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
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
func (b *backendSubscribeBlocks) SubscribeBlockDigestsFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	blockStatus flow.BlockStatus,
) subscription.Subscription[*flow.BlockDigest] {
	startHeight, err := b.blockTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subimpl.NewFailedSubscription[*flow.BlockDigest](err, "could not get start height from latest")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.buildReadyUpToHeight(blockStatus),
		b.blockDigestAtHeight,
	)

	sub := subimpl.NewSubscription[*flow.BlockDigest](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.finalizedBlockBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
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
func (b *backendSubscribeBlocks) SubscribeBlockDigestsFromStartHeight(
	ctx context.Context,
	startHeight uint64,
	blockStatus flow.BlockStatus,
) subscription.Subscription[*flow.BlockDigest] {
	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.buildReadyUpToHeight(blockStatus),
		b.blockDigestAtHeight,
	)

	sub := subimpl.NewSubscription[*flow.BlockDigest](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.finalizedBlockBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
}

// SubscribeBlockDigestsFromLatest streams finalized or sealed block digests starting at the latest sealed block,
// up until the latest available block digest. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block header as it becomes available.
//
// Each block digest are filtered by the provided block status, and only
// those block digests that match the status are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// If invalid parameters are provided, SubscribeBlockDigestsFromLatest will return a failed subscription.
func (b *backendSubscribeBlocks) SubscribeBlockDigestsFromLatest(
	ctx context.Context,
	blockStatus flow.BlockStatus,
) subscription.Subscription[*flow.BlockDigest] {
	startHeight, err := b.blockTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subimpl.NewFailedSubscription[*flow.BlockDigest](err, "could not get start height from latest")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.buildReadyUpToHeight(blockStatus),
		b.blockDigestAtHeight,
	)

	sub := subimpl.NewSubscription[*flow.BlockDigest](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.finalizedBlockBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
}

func (b *backendSubscribeBlocks) blockAtHeight(_ context.Context, height uint64) (*flow.Block, error) {
	// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("failed to retrieve block for height %d: %w", height, subscription.ErrBlockNotReady)
		}
		return nil, err
	}

	b.log.Trace().
		Hex("block_id", logging.ID(block.ID())).
		Uint64("height", height).
		Msgf("sending block info")

	return block, nil
}

func (b *backendSubscribeBlocks) headerAtHeight(_ context.Context, height uint64) (*flow.Header, error) {
	// since we are querying a finalized or sealed block header, we can use the height index and save an ID computation
	header, err := b.headers.ByHeight(height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("failed to retrieve block header for height %d: %w", height, subscription.ErrBlockNotReady)
		}
		return nil, err
	}

	b.log.Trace().
		Hex("block_id", logging.ID(header.ID())).
		Uint64("height", height).
		Msgf("sending block header info")

	return header, nil
}

func (b *backendSubscribeBlocks) blockDigestAtHeight(ctx context.Context, height uint64) (*flow.BlockDigest, error) {
	header, err := b.headerAtHeight(ctx, height)
	if err != nil {
		return nil, err
	}

	b.log.Trace().
		Hex("block_id", logging.ID(header.ID())).
		Uint64("height", height).
		Msgf("sending lightweight block info")

	return flow.NewBlockDigest(header.ID(), header.Height, time.UnixMilli(int64(header.Timestamp)).UTC()), nil
}

func (b *backendSubscribeBlocks) buildReadyUpToHeight(blockStatus flow.BlockStatus) func() (uint64, error) {
	return func() (uint64, error) {
		return b.blockTracker.GetHighestHeight(blockStatus)
	}
}

package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type backendSubscribeBlocks struct {
	log            zerolog.Logger
	state          protocol.State
	blocks         storage.Blocks
	Broadcaster    *engine.Broadcaster //TODO: Should be moved instead of public. Maybe to SubscriptionHandler
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	getStartHeight   GetStartHeightFunc
	getHighestHeight GetHighestHeight
}

func (b backendSubscribeBlocks) SubscribeBlocks(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	sub := subscription.NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getResponse(blockStatus))
	go subscription.NewStreamer(b.log, b.Broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

func (b backendSubscribeBlocks) getResponse(blockStatus flow.BlockStatus) subscription.GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		block, err := b.getBlock(ctx, height, blockStatus)
		if err != nil {
			return nil, fmt.Errorf("could not get block by height %d: %w", height, err)
		}

		b.log.Trace().
			Hex("block_id", logging.ID(block.ID())).
			Uint64("height", height).
			Msgf("sending block info")

		return block, nil
	}
}

// getBlock returns the block for the given block height.
// Expected errors during normal operation:
// - storage.ErrNotFound or execution_data.BlobNotFoundError: block for the given block height is not available.
func (b *backendSubscribeBlocks) getBlock(ctx context.Context, height uint64, expectedBlockStatus flow.BlockStatus) (*flow.Block, error) {
	// fail early if no notification has been received for the given block height.
	// note: it's possible for the data to exist in the data store before the notification is
	// received. this ensures a consistent view is available to all streams.
	if height > b.getHighestHeight() {
		return nil, fmt.Errorf("block %d is not available yet: %w", height, storage.ErrNotFound)
	}

	// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get block by height: %v", err)
	}

	sealed, err := b.state.Sealed().Head()
	if err != nil {
		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests, but do throw an exception which might cause a crash:
		// - It is unsafe to process requests if we have an internally bad state.
		// - We would like to avoid throwing an exception as a result of an Access API request by policy
		//   because this can cause DOS potential
		// - Since the protocol state is widely shared, we assume that in practice another component will
		//   observe the protocol state error and throw an exception.
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, status.Errorf(codes.Internal, "could not get latest sealed block: %v", err)
	}

	var blockStatus flow.BlockStatus
	if block.Header.Height > sealed.Height {
		blockStatus = flow.BlockStatusFinalized
	} else {
		blockStatus = flow.BlockStatusSealed
	}

	if blockStatus != expectedBlockStatus {
		return &flow.Block{}, nil
	}
	return block, nil
}

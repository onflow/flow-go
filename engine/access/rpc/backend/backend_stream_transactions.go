package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/common/rpc/convert"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendSubscribeTransactions struct {
	log            zerolog.Logger
	state          protocol.State
	blocks         storage.Blocks
	results        storage.LightTransactionResults
	Broadcaster    *engine.Broadcaster
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	messageIndex uint64

	getStartHeight   subscription.GetStartHeightFunc
	getHighestHeight subscription.GetHighestHeight
}

func (b *backendSubscribeTransactions) SendAndSubscribeTransactionStatuses(ctx context.Context, tx *flow.TransactionBody) subscription.Subscription {
	nextHeight, err := b.getStartHeight(tx.ReferenceBlockID, 0, flow.BlockStatusFinalized)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	b.messageIndex = 0
	sub := subscription.NewHeightBasedSubscription(
		b.sendBufferSize,
		nextHeight,
		func(ctx context.Context, height uint64) (interface{}, error) {
			return b.backendSubscribeTransactions(ctx, tx, height)
		},
	)

	go subscription.NewStreamer(b.log, b.Broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

func (b *backendSubscribeTransactions) backendSubscribeTransactions(ctx context.Context, tx *flow.TransactionBody, height uint64) (interface{}, error) {
	highestHeight, err := b.getHighestHeight(flow.BlockStatusFinalized)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	// fail early if no notification has been received for the given block height.
	// note: it's possible for the data to exist in the data store before the notification is
	// received. this ensures a consistent view is available to all streams.
	if height > highestHeight {
		return nil, fmt.Errorf("block %d is not available yet: %w", height, storage.ErrNotFound)
	}

	// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get block by height: %v", err)
	}

	result, err := b.results.ByBlockIDTransactionID(block.ID(), tx.ID())
	if err != nil {
		err = rpc.ConvertStorageError(err)
		if status.Code(err) != codes.NotFound {
			return nil, err
		}
	}

	var txStatus flow.TransactionStatus
	if result == nil {
		txStatus, err = b.deriveSubscribeTransactionStatus(tx, false, nil)
	} else {
		txStatus, err = b.deriveSubscribeTransactionStatus(tx, true, block)
	}

	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	response := &convert.TransactionSubscribeInfo{
		ID:           tx.ID(),
		Status:       txStatus,
		MessageIndex: b.messageIndex,
	}

	b.messageIndex++

	return response, nil
}

// deriveSubscribeTransactionStatus derives the transaction status based on current protocol state
// Error returns:
//   - state.ErrUnknownSnapshotReference - block referenced by transaction has not been found.
//   - all other errors are unexpected and potentially symptoms of internal implementation bugs or state corruption (fatal).
func (b *backendSubscribeTransactions) deriveSubscribeTransactionStatus(
	tx *flow.TransactionBody,
	executed bool,
	block *flow.Block,
) (flow.TransactionStatus, error) {
	if block == nil {
		// Not in a block, let's see if it's expired
		referenceBlock, err := b.state.AtBlockID(tx.ReferenceBlockID).Head()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}
		refHeight := referenceBlock.Height
		// get the latest finalized block from the state
		finalized, err := b.state.Final().Head()
		if err != nil {
			return flow.TransactionStatusUnknown, irrecoverable.NewExceptionf("failed to lookup final header: %w", err)
		}
		finalizedHeight := finalized.Height

		// if we haven't seen the expiry block for this transaction, it's not expired
		if !b.isExpiredSubscribe(refHeight, finalizedHeight) {
			return flow.TransactionStatusPending, nil
		}

		// At this point, we have seen the expiry block for the transaction.
		// This means that, if no collections  prior to the expiry block contain
		// the transaction, it can never be included and is expired.
		//
		// To ensure this, we need to have received all collections  up to the
		// expiry block to ensure the transaction did not appear in any.

		// the last full height is the height where we have received all
		// collections  for all blocks with a lower height
		fullHeight, err := b.blocks.GetLastFullBlockHeight()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}

		// if we have received collections  for all blocks up to the expiry block, the transaction is expired
		if b.isExpiredSubscribe(refHeight, fullHeight) {
			return flow.TransactionStatusExpired, nil
		}

		// tx found in transaction storage and collection storage but not in block storage
		// However, this will not happen as of now since the ingestion engine doesn't subscribe
		// for collections
		return flow.TransactionStatusPending, nil
	}

	if !executed {
		// If we've gotten here, but the block has not yet been executed, report it as only been finalized
		return flow.TransactionStatusFinalized, nil
	}

	// From this point on, we know for sure this transaction has at least been executed

	// get the latest sealed block from the State
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return flow.TransactionStatusUnknown, irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
	}

	if block.Header.Height > sealed.Height {
		// The block is not yet sealed, so we'll report it as only executed
		return flow.TransactionStatusExecuted, nil
	}

	// otherwise, this block has been executed, and sealed, so report as sealed
	return flow.TransactionStatusSealed, nil
}

// isExpired checks whether a transaction is expired given the height of the
// transaction's reference block and the height to compare against.
func (b *backendSubscribeTransactions) isExpiredSubscribe(refHeight, compareToHeight uint64) bool {
	if compareToHeight <= refHeight {
		return false
	}
	return compareToHeight-refHeight > flow.DefaultTransactionExpiry
}

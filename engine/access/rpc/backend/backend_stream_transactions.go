package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/module/counters"

	"github.com/onflow/flow-go/engine/common/rpc/convert"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// DeriveTransactionStatus is a function to derives the transaction status based on current protocol state
type DeriveTransactionStatus func(tx *flow.TransactionBody, executed bool, block *flow.Block) (flow.TransactionStatus, error)

type backendSubscribeTransactions struct {
	log            zerolog.Logger
	state          protocol.State
	blocks         storage.Blocks
	results        storage.LightTransactionResults
	Broadcaster    *engine.Broadcaster
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	getStartHeight          subscription.GetStartHeightFunc
	getHighestHeight        subscription.GetHighestHeight
	deriveTransactionStatus DeriveTransactionStatus
}

type TransactionSubscriptionMetadata struct {
	txID         flow.Identifier
	txBody       *flow.TransactionBody
	messageIndex counters.StrictMonotonousCounter
	blockWithTx  *flow.Block
}

func (b *backendSubscribeTransactions) SendAndSubscribeTransactionStatuses(ctx context.Context, tx *flow.TransactionBody) subscription.Subscription {
	nextHeight, err := b.getStartHeight(tx.ReferenceBlockID, 0, flow.BlockStatusFinalized)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	if b.deriveTransactionStatus == nil {
		return subscription.NewFailedSubscription(
			status.Errorf(codes.Internal, "failed to create transaction statuses subscription"),
			"DeriveTransactionStatus function must be initialized",
		)
	}

	txInfo := TransactionSubscriptionMetadata{
		txID:         tx.ID(),
		txBody:       tx,
		messageIndex: counters.NewMonotonousCounter(0),
		blockWithTx:  nil,
	}

	sub := subscription.NewHeightBasedSubscription(
		b.sendBufferSize,
		nextHeight,
		b.backendSubscribeTransactions(&txInfo),
	)

	go subscription.NewStreamer(b.log, b.Broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

func (b *backendSubscribeTransactions) backendSubscribeTransactions(txInfo *TransactionSubscriptionMetadata) func(context.Context, uint64) (interface{}, error) {
	return func(_ context.Context, height uint64) (interface{}, error) {
		executed := txInfo.blockWithTx != nil
		if !executed {
			highestHeight, err := b.getHighestHeight(flow.BlockStatusFinalized)

			if err != nil {
				return nil, fmt.Errorf("could not get highest height for block %d: %w", height, err)
			}

			// fail early if no notification has been received for the given block height.
			if height > highestHeight {
				return nil, fmt.Errorf("block %d is not available yet: %w", height, storage.ErrNotFound)
			}

			// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
			block, err := b.blocks.ByHeight(height)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "could not get block by height: %v", err)
			}

			result, err := b.results.ByBlockIDTransactionID(block.ID(), txInfo.txID)
			if err != nil {
				err = rpc.ConvertStorageError(err)
				if status.Code(err) != codes.NotFound {
					return nil, err
				}
			}

			if result != nil {
				txInfo.blockWithTx = block
			}
		}

		txStatus, err := b.deriveTransactionStatus(txInfo.txBody, executed, txInfo.blockWithTx)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		messageIndex := txInfo.messageIndex.Value()
		if ok := txInfo.messageIndex.Set(messageIndex + 1); !ok {
			b.log.Debug().Msg("message index already incremented")
		}

		return &convert.TransactionSubscribeInfo{
			ID:           txInfo.txID,
			Status:       txStatus,
			MessageIndex: messageIndex,
		}, nil
	}
}

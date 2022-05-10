package requester

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
)

type BlockExecutionDataConsumer func(blockHeight uint64, executionData *execution_data.BlockExecutionData)

type RequesterOption func(*RequesterConfig)

type RequesterConfig struct {
	MaxBlobSize          int
	RetryBaseDelay       time.Duration
	NumConcurrentWorkers int
}

type Requester struct {
	notifier   *notifier
	dispatcher *dispatcher
	fulfiller  *fulfiller
	handler    *handler

	finalizedBlockIDs chan flow.Identifier
	blocks            storage.Blocks

	logger zerolog.Logger

	cm *component.ComponentManager
	component.Component
}

func NewRequester(
	startHeight uint64,
	trackerStorage *tracker.Storage,
	blocks storage.Blocks,
	results storage.ExecutionResults,
	blobService network.BlobService,
	codec encoding.Codec,
	compressor network.Compressor,
	finalizationDistributor *pubsub.FinalizationDistributor,
	logger zerolog.Logger,
	metrics module.ExecutionDataRequesterMetrics,
	opts ...RequesterOption,
) (*Requester, error) {
	config := &RequesterConfig{
		MaxBlobSize:          execution_data.DefaultMaxBlobSize,
		RetryBaseDelay:       1 * time.Second,
		NumConcurrentWorkers: 8,
	}

	for _, opt := range opts {
		opt(config)
	}

	fulfilledHeight, err := trackerStorage.GetFulfilledHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest fulfilled height: %w", err)
	}

	logger = logger.With().Str("component", "execution_data_requester").Logger()
	notifier := newNotifier(logger)
	fulfiller := newFulfiller(fulfilledHeight, notifier, trackerStorage, logger, metrics)
	handler := newHandler(
		fulfiller,
		trackerStorage,
		blobService,
		execution_data.NewSerializer(codec, compressor),
		config.MaxBlobSize,
		config.RetryBaseDelay,
		config.NumConcurrentWorkers,
		logger,
		metrics,
	)
	dispatcher := newDispatcher(startHeight, blocks, results, handler, fulfiller, logger, metrics)

	r := &Requester{
		notifier:          notifier,
		dispatcher:        dispatcher,
		fulfiller:         fulfiller,
		handler:           handler,
		finalizedBlockIDs: make(chan flow.Identifier),
		blocks:            blocks,
		logger:            logger,
	}

	finalizationDistributor.AddOnBlockFinalizedConsumer(r.handleFinalizedBlock)

	r.cm = component.NewComponentManagerBuilder().
		AddWorker(r.runFulfiller).
		AddWorker(r.runHandler).
		AddWorker(r.runNotifier).
		AddWorker(r.runDispatcher(results, fulfilledHeight, startHeight)).
		AddWorker(r.processFinalizedBlocks).
		Build()
	r.Component = r.cm

	return r, nil
}

// AddConsumer adds a new block execution data consumer.
// The returned function can be used to remove the consumer.
func (r *Requester) AddConsumer(consumer BlockExecutionDataConsumer) (func() error, error) {
	return r.notifier.subscribe(&notificationSub{consumer})
}

// HandleReceipt handles a new Execution Receipt received from the network.
// TODO: once the network API has been refactored to support multiple engines per channel,
// we should allow the requester to subscribe to the receipt broadcast channel itself.
func (r *Requester) HandleReceipt(receipt *flow.ExecutionReceipt) {
	if util.CheckClosed(r.cm.ShutdownSignal()) {
		return
	}
	r.dispatcher.submitReceipt(receipt)
}

func (r *Requester) handleFinalizedBlock(block *model.Block) {
	if util.CheckClosed(r.cm.ShutdownSignal()) {
		return
	}
	r.finalizedBlockIDs <- block.BlockID
}

func (r *Requester) processFinalizedBlocks(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case blockID := <-r.finalizedBlockIDs:
			block, err := r.blocks.ByID(blockID)
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to retrieve finalized block %s: %w", blockID.String(), err))
			}
			r.dispatcher.submitFinalizedBlock(block)
		}
	}
}

func (r *Requester) runFulfiller(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	r.fulfiller.Start(ctx)
	if util.WaitClosed(ctx, r.fulfiller.Ready()) == nil {
		ready()
	}
	<-r.fulfiller.Done()
}

func (r *Requester) runHandler(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	r.handler.Start(ctx)
	if util.WaitClosed(ctx, r.handler.Ready()) == nil {
		ready()
	}
	<-r.handler.Done()
}

func (r *Requester) runNotifier(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	r.notifier.Start(ctx)
	if util.WaitClosed(ctx, r.handler.Ready()) == nil {
		ready()
	}
	<-r.notifier.Done()
}

func (r *Requester) runDispatcher(results storage.ExecutionResults, fulfilledHeight, sealedHeight uint64) component.ComponentWorker {
	return func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		for h := fulfilledHeight + 1; h < sealedHeight; h++ {
			blk, err := r.blocks.ByHeight(h)
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to retrieve sealed block at height %d: %w", h, err))
			}

			sealedResult, err := results.ByBlockID(blk.ID())
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to retrieve sealed result for block %s: %w", blk.ID().String(), err))
			}

			r.logger.Info().
				Str("result_id", sealedResult.ID().String()).
				Str("execution_data_id", sealedResult.ExecutionDataID.String()).
				Str("block_id", sealedResult.BlockID.String()).
				Uint64("block_height", h).
				Msg("pre-submitting job")

			r.fulfiller.submitSealedResult(sealedResult.ID())
			r.handler.submitJob(&job{
				ctx:             ctx,
				resultID:        sealedResult.ID(),
				executionDataID: sealedResult.ExecutionDataID,
				blockID:         sealedResult.BlockID,
				blockHeight:     h,
			})
		}

		r.dispatcher.Start(ctx)
		if util.WaitClosed(ctx, r.dispatcher.Ready()) == nil {
			ready()
		}
		<-r.dispatcher.Done()
	}
}

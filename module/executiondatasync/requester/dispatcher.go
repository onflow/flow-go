package requester

import (
	"context"
	"fmt"
	"sort"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage"
)

type resultInfo struct {
	resultID        flow.Identifier
	blockID         flow.Identifier // the ID of the executed block
	blockHeight     uint64          // the height of the executed block
	executionDataID flow.Identifier
	sealed          bool
}

// The purpose of this component is:
// * Dispatch requests to download execution data for sealed and unsealed execution results
// * Cancel requests to download execution data for unsealed execution results once a conflicting sealed execution result has been observed
// * Notifies fulfiller about sealed results so that it knows when a height can be fulfilled
// The component consumes broadcasted unsealed receipts and receipts included in finalized blocks
type dispatcher struct {
	// sealedHeight and jobs must be accessed only by the processResults goroutine
	sealedHeight uint64

	// jobs tracks results for which we have dispatched a request to download the corresponding
	// execution data. This is used to avoid dispatching the same request twice and to cancel
	// requests for unsealed results at a particular height once that height has been sealed.
	// blockHeight -> resultID -> cancel
	jobs map[uint64]map[flow.Identifier]context.CancelFunc

	receiptsIn         chan<- interface{}
	receiptsOut        <-chan interface{}
	finalizedBlocksIn  chan<- interface{}
	finalizedBlocksOut <-chan interface{}
	resultInfosIn      chan<- interface{}
	resultInfosOut     <-chan interface{}

	blocks  storage.Blocks
	results storage.ExecutionResults

	handler   *handler
	fulfiller *fulfiller

	logger  zerolog.Logger
	metrics module.ExecutionDataRequesterV2Metrics

	component.Component
}

func newDispatcher(
	sealedHeight uint64,
	blocks storage.Blocks,
	results storage.ExecutionResults,
	handler *handler,
	fulfiller *fulfiller,
	logger zerolog.Logger,
	metrics module.ExecutionDataRequesterV2Metrics,
) *dispatcher {
	receiptsIn, receiptsOut := util.UnboundedChannel()
	finalizedBlocksIn, finalizedBlocksOut := util.UnboundedChannel()
	resultInfosIn, resultInfosOut := util.UnboundedChannel()

	d := &dispatcher{
		sealedHeight:       sealedHeight,
		jobs:               make(map[uint64]map[flow.Identifier]context.CancelFunc),
		receiptsIn:         receiptsIn,
		receiptsOut:        receiptsOut,
		finalizedBlocksIn:  finalizedBlocksIn,
		finalizedBlocksOut: finalizedBlocksOut,
		resultInfosIn:      resultInfosIn,
		resultInfosOut:     resultInfosOut,
		blocks:             blocks,
		results:            results,
		handler:            handler,
		fulfiller:          fulfiller,
		logger:             logger.With().Str("subcomponent", "dispatcher").Logger(),
		metrics:            metrics,
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(d.processReceipts).
		AddWorker(d.processFinalizedBlocks).
		AddWorker(d.processResults).
		Build()

	d.Component = cm

	return d
}

func (d *dispatcher) submitReceipt(receipt *flow.ExecutionReceipt) {
	d.receiptsIn <- receipt
}

func (d *dispatcher) submitFinalizedBlock(block *flow.Block) {
	d.finalizedBlocksIn <- block
}

func (d *dispatcher) processReceipts(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case r := <-d.receiptsOut:
			receipt := r.(*flow.ExecutionReceipt)

			// TODO: The dispatcher should only dispatch jobs for results with corresponding block heights that
			// are near enough to the sealed height that, if they are for invalid results, would be cancelled
			// relatively quickly by falling beneath the sealed height. Otherwise, jobs for malicious receipts
			// could block a handler thread forever. Currently, we implement this by checking if the block exists
			// in local storage (and hence has been incorporated), but we may want to consider adding more
			// aggressive checks in the future since unfinalized subtrees of incorporated but uncertified blocks
			// are not guaranteed to be bounded in size.
			block, err := d.blocks.ByID(receipt.ExecutionResult.BlockID)
			if err != nil {
				d.logger.Debug().
					Err(err).
					Str("result_id", receipt.ExecutionResult.ID().String()).
					Str("receipt_id", receipt.ID().String()).
					Str("block_id", receipt.ExecutionResult.BlockID.String()).
					Msg("could not retrieve corresponding block for execution receipt, skipping")
				d.metrics.ReceiptSkipped()
				continue
			}

			d.resultInfosIn <- &resultInfo{
				resultID:        receipt.ExecutionResult.ID(),
				blockID:         receipt.ExecutionResult.BlockID,
				blockHeight:     block.Header.Height,
				executionDataID: receipt.ExecutionResult.ExecutionDataID,
			}
		}
	}
}

func (d *dispatcher) processFinalizedBlocks(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	if util.WaitClosed(ctx, d.fulfiller.Ready()) == nil {
		ready()
	} else {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case fb := <-d.finalizedBlocksOut:
			finalizedBlock := fb.(*flow.Block)
			rinfos := make([]*resultInfo, 0, len(finalizedBlock.Payload.Seals))
			for _, seal := range finalizedBlock.Payload.Seals {
				result, err := d.results.ByID(seal.ResultID)
				if err != nil {
					ctx.Throw(fmt.Errorf("could not retrieve sealed result: %w", err))
				}

				sealedBlock, err := d.blocks.ByID(seal.BlockID)
				if err != nil {
					ctx.Throw(fmt.Errorf("could not retrieve sealed block: %w", err))
				}

				rinfos = append(rinfos, &resultInfo{
					resultID:        seal.ResultID,
					blockID:         seal.BlockID,
					blockHeight:     sealedBlock.Header.Height,
					executionDataID: result.ExecutionDataID,
					sealed:          true,
				})
			}

			sort.Slice(rinfos, func(i, j int) bool { return rinfos[i].blockHeight < rinfos[j].blockHeight })

			for _, rinfo := range rinfos {
				d.fulfiller.submitSealedResult(rinfo.resultID, rinfo.blockHeight)
				d.resultInfosIn <- rinfo
			}
		}
	}
}

func (d *dispatcher) processResults(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	if util.WaitClosed(ctx, d.handler.Ready()) == nil {
		ready()
	} else {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ri := <-d.resultInfosOut:
			rInfo := ri.(*resultInfo)
			if rInfo.sealed {
				d.handleSealedResult(ctx, rInfo)
			} else {
				d.handleUnsealedResult(ctx, rInfo)
			}
		}
	}
}

func (d *dispatcher) dispatchJob(ctx irrecoverable.SignalerContext, resultID, executionDataID, blockID flow.Identifier, blockHeight uint64) context.CancelFunc {
	jobCtx, cancel := context.WithCancel(ctx)

	d.logger.Debug().
		Str("execution_data_id", executionDataID.String()).
		Str("result_id", resultID.String()).
		Str("block_id", blockID.String()).
		Uint64("block_height", blockHeight).
		Msg("dispatching job")

	d.handler.submitJob(&job{
		ctx:             jobCtx,
		executionDataID: executionDataID,
		blockID:         blockID,
		blockHeight:     blockHeight,
		resultID:        resultID,
	})

	return cancel
}

func (d *dispatcher) handleSealedResult(ctx irrecoverable.SignalerContext, rinfo *resultInfo) {
	if rinfo.blockHeight > d.sealedHeight {
		d.sealedHeight = rinfo.blockHeight
	}

	hmap, ok := d.jobs[rinfo.blockHeight]
	if ok {
		var requested bool

		// once we know the sealed execution data ID, we can
		// cancel all other jobs
		for resultID, cancel := range hmap {
			if rinfo.resultID == resultID {
				requested = true
			} else {
				// The dispatcher must cancel jobs for unsealed result IDs once they fall below the sealed height.
				// Otherwise jobs for malicious receipts could block a handler thread forever.
				cancel()
				d.metrics.RequestCanceled()
			}
		}

		delete(d.jobs, rinfo.blockHeight)

		// if the sealed execution data has already been requested,
		// there is nothing left to do
		if requested {
			return
		}
	}

	d.dispatchJob(ctx, rinfo.resultID, rinfo.executionDataID, rinfo.blockID, rinfo.blockHeight)
}

func (d *dispatcher) handleUnsealedResult(ctx irrecoverable.SignalerContext, rinfo *resultInfo) {
	if d.sealedHeight >= rinfo.blockHeight {
		// skip processing heights that are already sealed
		return
	}

	hmap, ok := d.jobs[rinfo.blockHeight]
	if !ok {
		hmap = make(map[flow.Identifier]context.CancelFunc)
		d.jobs[rinfo.blockHeight] = hmap
	} else if _, requested := hmap[rinfo.resultID]; requested {
		// if the execution data has already been requested,
		// there is nothing to do
		return
	}

	cancel := d.dispatchJob(ctx, rinfo.resultID, rinfo.executionDataID, rinfo.blockID, rinfo.blockHeight)
	hmap[rinfo.resultID] = cancel
}

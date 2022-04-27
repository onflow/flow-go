package requester

import (
	"context"
	"fmt"
	"sort"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage"
)

type resultInfo struct {
	resultID        flow.Identifier
	blockID         flow.Identifier
	blockHeight     uint64
	executionDataID flow.Identifier
	sealed          bool
}

type dispatcher struct {
	sealedHeight uint64
	jobs         map[uint64]map[flow.Identifier]context.CancelFunc

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

	component.Component
}

func newDispatcher(
	sealedHeight uint64,
	blocks storage.Blocks,
	results storage.ExecutionResults,
	handler *handler,
	fulfiller *fulfiller,
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
	// TODO: do we also want an upper bound on height?
	// For example, we could say that we ignore anything past the latest incorporated height?
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
			block, err := d.blocks.ByID(receipt.ExecutionResult.BlockID)
			if err != nil {
				// TODO: log
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
	<-d.fulfiller.Ready()
	ready()

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
				d.fulfiller.submitSealedResult(rinfo.resultID)
				d.resultInfosIn <- rinfo
			}
		}
	}
}

func (d *dispatcher) processResults(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	<-d.handler.Ready()
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case ri := <-d.resultInfosOut:
			rInfo := ri.(*resultInfo)
			if rInfo.sealed {
				d.handleSealedResult(ctx, rInfo)
			} else {
				d.handleResult(ctx, rInfo)
			}
		}
	}
}

func (d *dispatcher) dispatchJob(ctx irrecoverable.SignalerContext, resultID, executionDataID, blockID flow.Identifier, blockHeight uint64) context.CancelFunc {
	jobCtx, cancel := context.WithCancel(ctx)

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
	d.sealedHeight = rinfo.blockHeight

	hmap, ok := d.jobs[rinfo.blockHeight]
	if ok {
		var requested bool

		// once we know the sealed execution data ID, we can
		// cancel all other jobs
		for resultID, cancel := range hmap {
			if rinfo.resultID == resultID {
				requested = true
			} else {
				cancel()
			}
		}

		delete(d.jobs, d.sealedHeight)

		// if the sealed execution data has already been requested,
		// there is nothing left to do
		if requested {
			return
		}
	}

	d.dispatchJob(ctx, rinfo.resultID, rinfo.executionDataID, rinfo.blockID, rinfo.blockHeight)
}

func (d *dispatcher) handleResult(ctx irrecoverable.SignalerContext, rinfo *resultInfo) {
	if d.sealedHeight >= rinfo.blockHeight {
		// TODO: log something
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

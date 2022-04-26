package requester

import (
	"context"
	"fmt"
	"sort"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type resultInfo struct {
	resultID        flow.Identifier
	blockID         flow.Identifier
	blockHeight     uint64
	executionDataID flow.Identifier
	sealed          bool
}

type Dispatcher struct {
	sealedHeight uint64
	jobs         map[uint64]map[flow.Identifier]context.CancelFunc

	receipts        chan *flow.ExecutionReceipt // TODO: use unbounded channel
	finalizedBlocks chan flow.Identifier        // TODO: unbounded channel
	resultInfos     chan *resultInfo            // TODO: unbounded channel

	blocks  storage.Blocks
	results storage.ExecutionResults

	handler *handler

	logger zerolog.Logger
}

func (d *Dispatcher) HandleReceipt(receipt *flow.ExecutionReceipt) {
	// TODO: do we also want an upper bound on height?
	// For example, we could say that we ignore anything past the latest incorporated height?
	d.receipts <- receipt
}

func (d *Dispatcher) HandleFinalizedBlock(block *model.Block) {
	d.finalizedBlocks <- block.BlockID
}

func (d *Dispatcher) processReceipts(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case receipt := <-d.receipts:
			block, err := d.blocks.ByID(receipt.ExecutionResult.BlockID)
			if err != nil {
				// TODO: log
				continue
			}

			d.resultInfos <- &resultInfo{
				resultID:        receipt.ExecutionResult.ID(),
				blockID:         receipt.ExecutionResult.BlockID,
				blockHeight:     block.Header.Height,
				executionDataID: receipt.ExecutionResult.ExecutionDataID,
			}
		}
	}
}

func (d *Dispatcher) processFinalizedBlocks(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case blockID := <-d.finalizedBlocks:
			finalizedBlock, err := d.blocks.ByID(blockID)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not retrieve finalized block: %w", err))
			}

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
				d.resultInfos <- rinfo
			}
		}
	}
}

func (d *Dispatcher) processResults(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case rinfo := <-d.resultInfos:
			if rinfo.sealed {
				d.handleSealedResult(ctx, rinfo)
			} else {
				d.handleResult(ctx, rinfo)
			}
		}
	}
}

func (d *Dispatcher) dispatchJob(ctx irrecoverable.SignalerContext, resultID, executionDataID, blockID flow.Identifier, blockHeight uint64) context.CancelFunc {
	jobCtx, cancel := context.WithCancel(ctx)

	d.handler.submit(&job{
		ctx:             jobCtx,
		executionDataID: executionDataID,
		blockID:         blockID,
		blockHeight:     blockHeight,
		resultID:        resultID,
	})

	return cancel
}

func (d *Dispatcher) handleSealedResult(ctx irrecoverable.SignalerContext, rinfo *resultInfo) {
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

func (d *Dispatcher) handleResult(ctx irrecoverable.SignalerContext, rinfo *resultInfo) {
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

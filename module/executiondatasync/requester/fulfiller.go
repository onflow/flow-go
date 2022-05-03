package requester

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
)

type jobResult struct {
	executionData *execution_data.BlockExecutionData
	resultID      flow.Identifier
	blockHeight   uint64
	err           error
}

type fulfiller struct {
	jobResults    map[uint64]map[flow.Identifier]*jobResult
	sealedResults map[uint64]flow.Identifier
	fulfilled     map[uint64]*jobResult

	fulfilledHeight uint64
	sealedHeight    uint64

	jobsIn     chan<- interface{}
	jobsOut    <-chan interface{}
	resultsIn  chan<- interface{}
	resultsOut <-chan interface{}

	notifier *notifier
	storage  *tracker.Storage

	component.Component
}

func newFulfiller(
	fulfilledHeight uint64,
	notifier *notifier,
	storage *tracker.Storage,
) *fulfiller {
	jobsIn, jobsOut := util.UnboundedChannel()
	resultsIn, resultsOut := util.UnboundedChannel()

	f := &fulfiller{
		jobResults:      make(map[uint64]map[flow.Identifier]*jobResult),
		sealedResults:   make(map[uint64]flow.Identifier),
		fulfilled:       make(map[uint64]*jobResult),
		jobsIn:          jobsIn,
		jobsOut:         jobsOut,
		resultsIn:       resultsIn,
		resultsOut:      resultsOut,
		fulfilledHeight: fulfilledHeight,
		sealedHeight:    fulfilledHeight,
		notifier:        notifier,
		storage:         storage,
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(f.loop).
		Build()

	f.Component = cm

	return f
}

func (f *fulfiller) submitJobResult(j *jobResult) {
	f.jobsIn <- j
}

func (f *fulfiller) submitSealedResult(resultID flow.Identifier) {
	f.resultsIn <- resultID
}

func (f *fulfiller) fulfill() {
loop:
	if j, ok := f.fulfilled[f.fulfilledHeight+1]; ok {
		f.fulfilledHeight++
		delete(f.fulfilled, f.fulfilledHeight)
		delete(f.sealedResults, f.fulfilledHeight)
		f.notifier.notify(j.blockHeight, j.executionData)

		goto loop
	}

	f.storage.SetFulfilledHeight(f.fulfilledHeight)
}

func (f *fulfiller) handleJobResult(ctx irrecoverable.SignalerContext, j *jobResult) {
	if j.blockHeight <= f.fulfilledHeight {
		return
	}

	if j.blockHeight <= f.sealedHeight {
		if j.resultID != f.sealedResults[j.blockHeight] {
			return
		}

		if j.err != nil {
			ctx.Throw(fmt.Errorf("failed to get sealed execution data for block height %d: %w", j.blockHeight, j.err))
		}

		f.fulfilled[j.blockHeight] = j

		if j.blockHeight == f.fulfilledHeight+1 {
			f.fulfill()
		}
	} else {
		jmap, ok := f.jobResults[j.blockHeight]
		if !ok {
			jmap = make(map[flow.Identifier]*jobResult)
			f.jobResults[j.blockHeight] = jmap
		}

		jmap[j.resultID] = j
	}
}

func (f *fulfiller) handleSealedResult(ctx irrecoverable.SignalerContext, sealedResultID flow.Identifier) {
	f.sealedHeight++
	f.sealedResults[f.sealedHeight] = sealedResultID

	if completedJobs, ok := f.jobResults[f.sealedHeight]; ok {
		delete(f.jobResults, f.sealedHeight)

		if j, ok := completedJobs[sealedResultID]; ok {
			if j.err != nil {
				ctx.Throw(fmt.Errorf("failed to get sealed execution data for block height %d: %w", j.blockHeight, j.err))
			}

			f.fulfilled[f.sealedHeight] = j

			if f.sealedHeight == f.fulfilledHeight+1 {
				f.fulfill()
			}
		}
	}
}

func (f *fulfiller) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	if util.WaitClosed(ctx, f.notifier.Ready()) == nil {
		ready()
	} else {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case j := <-f.jobsOut:
			f.handleJobResult(ctx, j.(*jobResult))
		case sealedResultID := <-f.resultsOut:
			f.handleSealedResult(ctx, sealedResultID.(flow.Identifier))
		}
	}
}

package requester

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type completedJob struct {
	executionData *execution_data.ExecutionData
	resultID      flow.Identifier
	blockHeight   uint64
}

type fulfiller struct {
	completedJobs map[uint64]map[flow.Identifier]*completedJob
	sealedResults map[uint64]flow.Identifier
	fulfilled     map[uint64]*completedJob

	fulfilledHeight uint64
	sealedHeight    uint64

	jobChan    chan *completedJob   // TODO: unbounded
	resultChan chan flow.Identifier // TODO: unbounded

	notifier *notifier
	storage  *tracker.Storage
}

func (f *fulfiller) submit(j *completedJob) {

}

func (f *fulfiller) fulfill() {
loop:
	if j, ok := f.fulfilled[f.fulfilledHeight+1]; ok {
		f.fulfilledHeight++
		delete(f.fulfilled, f.fulfilledHeight)
		delete(f.sealedResults, f.fulfilledHeight)
		f.notifier.notify(j)

		goto loop
	}

	f.storage.SetFulfilledHeight(f.fulfilledHeight)
}

func (f *fulfiller) handleCompletedJob(j *completedJob) {
	if j.blockHeight <= f.fulfilledHeight {
		return
	}

	if j.blockHeight <= f.sealedHeight {
		if j.resultID != f.sealedResults[j.blockHeight] {
			return
		}

		f.fulfilled[j.blockHeight] = j

		if j.blockHeight == f.fulfilledHeight+1 {
			f.fulfill()
		}
	} else {
		jmap, ok := f.completedJobs[j.blockHeight]
		if !ok {
			jmap = make(map[flow.Identifier]*completedJob)
			f.completedJobs[j.blockHeight] = jmap
		}

		jmap[j.resultID] = j
	}
}

func (f *fulfiller) handleSealedResult(sealedResultID flow.Identifier) {
	f.sealedHeight++
	f.sealedResults[f.sealedHeight] = sealedResultID

	if completedJobs, ok := f.completedJobs[f.sealedHeight]; ok {
		delete(f.completedJobs, f.sealedHeight)

		if j, ok := completedJobs[sealedResultID]; ok {
			f.fulfilled[f.sealedHeight] = j

			if f.sealedHeight == f.fulfilledHeight+1 {
				f.fulfill()
			}
		}
	}
}

func (f *fulfiller) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case j := <-f.jobChan:
			f.handleCompletedJob(j)
		case sealedResultID := <-f.resultChan:
			f.handleSealedResult(sealedResultID)
		}
	}
}

package requester

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
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

	logger  zerolog.Logger
	metrics module.ExecutionDataRequesterMetrics

	notifier *notifier
	storage  tracker.Storage

	component.Component
}

func newFulfiller(
	fulfilledHeight uint64,
	notifier *notifier,
	storage tracker.Storage,
	logger zerolog.Logger,
	metrics module.ExecutionDataRequesterMetrics,
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
		logger:          logger.With().Str("subcomponent", "fulfiller").Logger(),
		metrics:         metrics,
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

func (f *fulfiller) fulfill() error {
loop:
	if j, ok := f.fulfilled[f.fulfilledHeight+1]; ok {
		f.fulfilledHeight++
		delete(f.fulfilled, f.fulfilledHeight)
		delete(f.sealedResults, f.fulfilledHeight)
		f.notifier.notify(j.blockHeight, j.executionData)

		goto loop
	}

	f.logger.Debug().Uint64("height", f.fulfilledHeight).Msg("updating fulfilled height")

	err := f.storage.SetFulfilledHeight(f.fulfilledHeight)
	if err == nil {
		f.metrics.FulfilledHeight(f.fulfilledHeight)
	}
	return err
}

func (f *fulfiller) handleJobResult(ctx irrecoverable.SignalerContext, j *jobResult) {
	if j.blockHeight <= f.fulfilledHeight {
		return
	}

	if j.blockHeight <= f.sealedHeight {
		if j.resultID != f.sealedResults[j.blockHeight] {
			f.logger.Debug().
				Str("result_id", j.resultID.String()).
				Str("block_id", j.executionData.BlockID.String()).
				Uint64("block_height", j.blockHeight).
				Msg("dropping job result")
			f.metrics.ResultDropped()
			return
		}

		if j.err != nil {
			// sealed execution data failed with non-retryable error
			ctx.Throw(fmt.Errorf("failed to get sealed execution data for block height %d: %w", j.blockHeight, j.err))
		}

		f.fulfilled[j.blockHeight] = j

		if j.blockHeight == f.fulfilledHeight+1 {
			if err := f.fulfill(); err != nil {
				ctx.Throw(fmt.Errorf("failed to fulfill: %w", err))
			}
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
		// once we know the sealed result ID, we can discard all unneeded job results
		delete(f.jobResults, f.sealedHeight)

		for resultID, j := range completedJobs {
			if resultID == sealedResultID {
				if j.err != nil {
					// sealed execution data failed with non-retryable error
					ctx.Throw(fmt.Errorf("failed to get sealed execution data for block height %d: %w", j.blockHeight, j.err))
				}

				f.fulfilled[f.sealedHeight] = j

				if f.sealedHeight == f.fulfilledHeight+1 {
					if err := f.fulfill(); err != nil {
						ctx.Throw(fmt.Errorf("failed to fulfill: %w", err))
					}
				}
			} else {
				f.logger.Debug().
					Str("result_id", resultID.String()).
					Str("block_id", j.executionData.BlockID.String()).
					Uint64("block_height", j.blockHeight).
					Msg("discarding job result")
				f.metrics.ResultDropped()
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

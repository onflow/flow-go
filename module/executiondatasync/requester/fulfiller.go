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
	blockHeight   uint64 // height of the executed block
	err           error  // err is set when a critical, non-retryable error was encountered while downloading execution data for this result
}

type sealedResult struct {
	resultID    flow.Identifier
	blockHeight uint64
}

// The fulfiller consumes sealed results from the dispatcher and
// completed execution data download jobs from the handler.
// A block is "fulfilled" when it has been sealed and we have
// downloaded the sealed execution data for that block.
// The job of the fulfiller is to detect when a new block has been
// fulfilled, and:
// * notify subscribers via the notifier
// * update the fulfilled height in the execution state download tracker
type fulfiller struct {
	// jobResults tracks all completed jobs for unsealed heights, including potentially for
	// invalid results which will never be sealed
	// height -> resultID -> jobResult
	jobResults map[uint64]map[flow.Identifier]*jobResult

	// sealedResults tracks the result ID which is sealed at each height
	sealedResults map[uint64]flow.Identifier

	// fulfilled tracks the completed, successful job for the most recent sealed heights
	// used to trigger notifications
	fulfilled map[uint64]*jobResult

	fulfilledHeight uint64
	sealedHeight    uint64

	jobsIn     chan<- interface{}
	jobsOut    <-chan interface{}
	resultsIn  chan<- interface{}
	resultsOut <-chan interface{}

	logger  zerolog.Logger
	metrics module.ExecutionDataRequesterV2Metrics

	notifier *notifier
	storage  tracker.Storage

	component.Component
}

func newFulfiller(
	fulfilledHeight uint64,
	notifier *notifier,
	storage tracker.Storage,
	logger zerolog.Logger,
	metrics module.ExecutionDataRequesterV2Metrics,
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

// submitSealedResult notifies the fulfiller of a new sealed result ID.
// This function MUST be called exactly once for each block height, and
// in the correct order.
func (f *fulfiller) submitSealedResult(resultID flow.Identifier, blockHeight uint64) {
	f.resultsIn <- sealedResult{resultID, blockHeight}
}

func (f *fulfiller) fulfill() error {
	for {
		j, ok := f.fulfilled[f.fulfilledHeight+1]
		if !ok {
			break
		}

		f.fulfilledHeight++
		delete(f.fulfilled, f.fulfilledHeight)
		delete(f.sealedResults, f.fulfilledHeight)
		f.notifier.notify(j.blockHeight, j.executionData)
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
				Uint64("block_height", j.blockHeight).
				Msg("dropping job result")
			f.metrics.ResponseDropped()
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

func (f *fulfiller) handleSealedResult(ctx irrecoverable.SignalerContext, sr sealedResult) {
	f.sealedHeight = sr.blockHeight
	f.sealedResults[f.sealedHeight] = sr.resultID

	if completedJobs, ok := f.jobResults[f.sealedHeight]; ok {
		// once we know the sealed result ID, we can discard all unneeded job results
		delete(f.jobResults, f.sealedHeight)

		for resultID, j := range completedJobs {
			if resultID == sr.resultID {
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
				f.metrics.ResponseDropped()
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
			f.handleSealedResult(ctx, sealedResultID.(sealedResult))
		}
	}
}

package events

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/tracker"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// ProcessLatestFinalizedBlock is invoked when a new block is finalized.
// It is possible that blocks will be skipped.
type ProcessLatestFinalizedBlock func(block *model.Block) error

// FinalizationActor is an event responder worker which can be embedded in a component
// to simplify the plumbing required to respond to block finalization events.
// This worker is designed to respond to a newly finalized blocks on a best-effort basis,
// meaning that it may skip blocks when finalization occurs more quickly.
// CAUTION: This is suitable for use only when the handler can tolerate skipped blocks.
type FinalizationActor struct {
	newestFinalized *tracker.NewestBlockTracker
	notifier        engine.Notifier
	handler         ProcessLatestFinalizedBlock
}

var _ hotstuff.FinalizationConsumer = (*FinalizationActor)(nil)

// NewFinalizationActor creates a new FinalizationActor, and returns the worker routine
// and event consumer required to operate it.
// The caller MUST:
//   - start the returned component.ComponentWorker function
//   - subscribe the returned FinalizationActor to ProcessLatestFinalizedBlock events
func NewFinalizationActor(handler ProcessLatestFinalizedBlock) (*FinalizationActor, component.ComponentWorker) {
	actor := &FinalizationActor{
		newestFinalized: tracker.NewNewestBlockTracker(),
		notifier:        engine.NewNotifier(),
		handler:         handler,
	}
	return actor, actor.workerLogic
}

// workerLogic is the worker function exposed by the FinalizationActor. It should be
// attached to a ComponentBuilder by the higher-level component.
// It processes each new finalized block by invoking the ProcessLatestFinalizedBlock callback.
func (actor *FinalizationActor) workerLogic(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	blockFinalizedSignal := actor.notifier.Channel()

	for {
		select {
		case <-doneSignal:
			return
		case <-blockFinalizedSignal:
			block := actor.newestFinalized.NewestBlock()
			err := actor.handler(block)
			if err != nil {
				ctx.Throw(err)
				return
			}
		}
	}
}

// OnFinalizedBlock receives block finalization events. It updates the newest finalized
// block tracker and notifies the worker thread.
func (actor *FinalizationActor) OnFinalizedBlock(block *model.Block) {
	if actor.newestFinalized.Track(block) {
		actor.notifier.Notify()
	}
}

func (actor *FinalizationActor) OnBlockIncorporated(*model.Block)                   {}
func (actor *FinalizationActor) OnDoubleProposeDetected(*model.Block, *model.Block) {}

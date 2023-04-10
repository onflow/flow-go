package events

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/tracker"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type OnBlockFinalized func(block *model.Block) error

// FinalizationActor is an event responder worker which can be embedded in a component
// to simplify the plumbing required to respond to block finalization events.
// This worker is designed to respond to a newly finalized blocks on a best-effort basis,
// meaning that it may skip blocks when finalization occurs more quickly.
// CAUTION: This is suitable for use only when the handler can tolerate skipped blocks.
type FinalizationActor struct {
	log             zerolog.Logger
	newestFinalized *tracker.NewestBlockTracker
	notifier        engine.Notifier
	handler         OnBlockFinalized
}

func NewFinalizationActor(log zerolog.Logger, handler OnBlockFinalized) component.ComponentWorker {
	actor := &FinalizationActor{
		log:             log.With().Str("worker", "finalization_actor").Logger(),
		newestFinalized: tracker.NewNewestBlockTracker(),
		notifier:        engine.NewNotifier(),
		handler:         handler,
	}
	return actor.worker
}

func (actor *FinalizationActor) worker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	blockFinalizedSignal := actor.notifier.Channel()

	for {
		select {
		case <-doneSignal:
			return
		case <-blockFinalizedSignal:
			block := actor.newestFinalized.NewestBlock()
			err := actor.handler(actor.newestFinalized.NewestBlock())
			if err != nil {
				actor.log.Err(err).Msgf("FinalizationActor encountered irrecoverable error at block (id=%x, view=%d)", block.BlockID, block.View)
				ctx.Throw(err)
				return
			}
		}
	}
}

func (actor *FinalizationActor) OnBlockFinalized(block *model.Block) {
	actor.newestFinalized.Track(block)
	actor.notifier.Notify()
}

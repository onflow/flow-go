package events

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/tracker"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// OnBlockFinalized is invoked when a new block is finalized. It is possible that
// blocks will be skipped.
type OnBlockFinalized func(block *model.Block) error

// FinalizationActor is an event responder worker which can be embedded in a component
// to simplify the plumbing required to respond to block finalization events.
// This worker is designed to respond to a newly finalized blocks on a best-effort basis,
// meaning that it may skip blocks when finalization occurs more quickly.
// CAUTION: This is suitable for use only when the handler can tolerate skipped blocks.
type FinalizationActor struct {
	newestFinalized *tracker.NewestBlockTracker
	notifier        engine.Notifier
	handler         OnBlockFinalized
}

// NewFinalizationActor creates a new FinalizationActor and subscribes it to the given event distributor.
func NewFinalizationActor(distributor *pubsub.FinalizationDistributor) *FinalizationActor {
	actor := NewUnsubscribedFinalizationActor()
	distributor.AddOnBlockFinalizedConsumer(actor.OnBlockFinalized)
	return actor
}

// NewUnsubscribedFinalizationActor creates a new FinalizationActor. The caller
// is responsible for subscribing the actor.
func NewUnsubscribedFinalizationActor() *FinalizationActor {
	actor := &FinalizationActor{
		newestFinalized: tracker.NewNewestBlockTracker(),
		notifier:        engine.NewNotifier(),
		handler:         nil, // set with CreateWorker
	}
	return actor
}

// CreateWorker embeds the OnBlockFinalized handler function into the actor, which
// means it is ready for use. A worker function is returned which should be added
// to a ComponentBuilder during construction of the higher-level component.
// One FinalizationActor instance provides exactly one worker, so CreateWorker will
// panic if it is called more than once.
func (actor *FinalizationActor) CreateWorker(handler OnBlockFinalized) component.ComponentWorker {
	if actor.handler != nil {
		panic("invoked CreatedWorker twice")
	}
	actor.handler = handler
	return actor.worker
}

// worker is the worker function exposed by the FinalizationActor. It should be
// attached to a ComponentBuilder by the higher-level component using CreateWorker.
// It processes each new finalized block by invoking the OnBlockFinalized callback.
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
			err := actor.handler(block)
			if err != nil {
				ctx.Throw(err)
				return
			}
		}
	}
}

// OnBlockFinalized receives block finalization events. It updates the newest finalized
// block tracker and notifies the worker thread.
func (actor *FinalizationActor) OnBlockFinalized(block *model.Block) {
	if actor.newestFinalized.Track(block) {
		actor.notifier.Notify()
	}
}

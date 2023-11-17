package storehouse

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// RegisterEngine is a wrapper for RegisterStore in order to make Block Finalization process
// non-blocking.
type RegisterEngine struct {
	*component.ComponentManager
	store                *RegisterStore
	finalizationNotifier engine.Notifier
}

func NewRegisterEngine(store *RegisterStore) *RegisterEngine {
	e := &RegisterEngine{
		store:                store,
		finalizationNotifier: engine.NewNotifier(),
	}

	// Add workers
	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.finalizationProcessingLoop).
		Build()
	return e
}

// OnBlockFinalized will create a single goroutine to notify register store
// when a block is finalized.
// This call is non-blocking in order to avoid blocking the consensus
func (e *RegisterEngine) OnBlockFinalized(*model.Block) {
	e.finalizationNotifier.Notify()
}

// finalizationProcessingLoop notify the register store when a block is finalized
// and handle the error if any
func (e *RegisterEngine) finalizationProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	notifier := e.finalizationNotifier.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			err := e.store.OnBlockFinalized()
			if err != nil {
				ctx.Throw(fmt.Errorf("could not process finalized block: %w", err))
			}
		}
	}
}

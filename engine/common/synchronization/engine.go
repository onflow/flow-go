// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
)

// Engine is the synchronization engine, responsible for synchronizing chain state.
type Engine struct {
	unit *engine.Unit
	log  zerolog.Logger
}

// New creates a new consensus propagation engine.
func New(
	log zerolog.Logger,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit: engine.NewUnit(),
		log:  log.With().Str("engine", "consensus").Logger(),
	}

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.checkLoop)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// checkLoop will regularly scan for items that need requesting.
func (e *Engine) checkLoop() {
	poll := time.NewTicker(1 * time.Second)
	// scan := time.NewTicker(10 * time.Second)

CheckLoop:
	for {
		select {
		case <-e.unit.Quit():
			break CheckLoop
		case <-poll.C:
			// remove the following two lines fixes the tests. why?
			e.unit.Lock()
			defer e.unit.Unlock()

			// case <-scan.C:
			// 	e.unit.Lock()
			// 	defer e.unit.Unlock()
		}
	}

	// some minor cleanup
	// scan.Stop()
	poll.Stop()
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package compliance

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
)

// Engine is the consensus engine, responsible for handling communication for
// the embedded consensus algorithm.
type Engine struct {
	unit *engine.Unit   // used to control startup/shutdown
	log  zerolog.Logger // used to log relevant actions with context
	sync *synchronization.Engine
}

// New creates a new consensus propagation engine.
func New(
	log zerolog.Logger,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit: engine.NewUnit(),
		log:  log.With().Str("engine", "consensus").Logger(),
		sync: nil, // use `WithSynchronization`
	}

	return e, nil
}

// WithSynchronization adds the synchronization engine responsible for bringing the node
// up to speed to the compliance engine.
func (e *Engine) WithSynchronization(sync *synchronization.Engine) *Engine {
	e.sync = sync
	return e
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.sync == nil {
		panic("must initialize compliance engine with synchronization module")
	}
	return e.unit.Ready(func() {
		<-e.sync.Ready()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		e.log.Info().Msg("sync Done")
		<-e.sync.Done()
		e.log.Info().Msg("hotstuff Done")
	})
}

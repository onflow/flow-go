// Package provider implements an engine for responding to requests for
// transactions that have been formed into collections and guaranteed.
package provider

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
)

// Engine is the collection provider engine, which responds to requests for
// transactions that have been guaranteed.
type Engine struct {
	log   zerolog.Logger
	con   network.Conduit
	me    module.Local
	state protocol.State
	// TODO storage provider for transactions/guaranteed collections
}

func New(log zerolog.Logger, net module.Network, me module.Local, state protocol.State) (*Engine, error) {
	e := &Engine{
		log:   log.With().Str("engine", "provider").Logger(),
		me:    me,
		state: state,
	}

	con, err := net.Register(engine.CollectionProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started.
func (e *Engine) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a done channel that is closed once the engine has fully stopped.
// TODO describe conditions under which engine is done
func (e *Engine) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		close(done)
	}()
	return done
}

// Submit allows us to submit local events to the propagation engine. The
// function logs errors internally, rather than returning it, which allows other
// engines to submit events in a non-blocking way by using a goroutine.
func (e *Engine) Submit(event interface{}) {
	err := e.Process(e.me.NodeID(), event)
	if err != nil {
		e.log.Error().Err(err).Msg("could not process local event")
	}
}

// Process processes the given propagation engine event. Events that are given
// to this function originate within the propagation engine on the node with the
// given origin ID.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	var err error
	switch event.(type) {
	// TODO transactionRequest, transactionRangeRequest
	default:
		err = fmt.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return fmt.Errorf("could not process event: %w", err)
	}
	return nil
}

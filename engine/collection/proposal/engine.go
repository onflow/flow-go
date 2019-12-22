// Package proposal implements an engine for proposing and guaranteeing
// collections and submitting them to consensus nodes.
package proposal

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
)

// The period at which the engine will propose a new collection based on the
// contents of the txpool.
const proposalPeriod = time.Second * 5

// Engine is the collection proposal engine, which packages pending
// transactions into collections and sends them to consensus nodes.
type Engine struct {
	unit  *engine.Unit
	log   zerolog.Logger
	con   network.Conduit
	me    module.Local
	state protocol.State
	pool  module.TransactionPool
	// TODO storage provider for transactions/guaranteed collections
}

func New(log zerolog.Logger, net module.Network, me module.Local, state protocol.State) (*Engine, error) {
	e := &Engine{
		unit:  engine.NewUnit(),
		log:   log.With().Str("engine", "proposal").Logger(),
		me:    me,
		state: state,
	}

	con, err := net.Register(engine.CollectionProposal, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.propose)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// TODO describe conditions under which engine is done
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			e.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process processes events for the proposal engine on the collection node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch event.(type) {
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) propose() {

	ticker := time.NewTicker(proposalPeriod)

	for {
		select {
		case <-ticker.C:
			// TODO propose a new block and send to consensus nodes
		case <-e.unit.Quit():
			return
		}
	}
}

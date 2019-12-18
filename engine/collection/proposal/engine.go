// Package proposal implements an engine for proposing and guaranteeing
// collections and submitting them to consensus nodes.
package proposal

import (
	"fmt"
	"sync"
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
	log   zerolog.Logger
	con   network.Conduit
	me    module.Local
	state protocol.State
	pool  module.TransactionPool
	// TODO storage provider for transactions/guaranteed collections

	// TODO replace with engine.Unit
	stop    chan struct{}  // used to stop the proposer goroutine
	stopped sync.WaitGroup // used to indicate that all goroutines have stopped
}

func New(log zerolog.Logger, net module.Network, state protocol.State, me module.Local) (*Engine, error) {
	e := &Engine{
		log:   log.With().Str("engine", "proposal").Logger(),
		me:    me,
		state: state,
		//pool:  pool,
		stop: make(chan struct{}),
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
	ready := make(chan struct{})
	go func() {
		go e.start()
		close(ready)
	}()
	return ready
}

// Done returns a done channel that is closed once the engine has fully stopped.
// TODO describe conditions under which engine is done
func (e *Engine) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		close(e.stop)
		e.stopped.Wait()
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
	default:
		err = fmt.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return fmt.Errorf("could not process event: %w", err)
	}
	return nil
}

func (e *Engine) start() {
	e.stopped.Add(1)
	defer e.stopped.Done()

	ticker := time.NewTicker(proposalPeriod)

	for {
		select {
		case <-ticker.C:
			// TODO propose a new block and send to consensus nodes
		case <-e.stop:
			return
		}
	}
}

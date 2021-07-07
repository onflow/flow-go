package relay

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/rs/zerolog"
)

type Engine struct {
	unit    *engine.Unit   // used to manage concurrency & shutdown
	log     zerolog.Logger // used to log relevant actions with context
	me      module.Local
	conduit network.Conduit
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
) (*Engine, error) {
	e := &Engine{
		unit: engine.NewUnit(),
		log:  log.With().Str("engine", "relay").Logger(),
		me:   me,
	}

	conduit, err := net.Register(engine.Relay, e)
	if err != nil {
		return nil, fmt.Errorf("could not register relay engine: %w", err)
	}
	e.conduit = conduit

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the relay engine, ... TODO
func (e *Engine) Ready() <-chan struct{} {
	// return e.unit.Ready(func() {
	// 	<-e.follower.Ready()
	// })

	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the relay engine, ... TODO
func (e *Engine) Done() <-chan struct{} {
	// return e.unit.Done(func() {
	// 	<-e.follower.Done()
	// })

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
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID
// in a blocking manner. It returns the potential processing error when
// done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	err := e.conduit.Publish(event)
	if err != nil {
		return fmt.Errorf("could not relay message: %w", err)
	}

	return nil
}

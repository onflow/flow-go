package match

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
)

type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	me       module.Local
	verifier network.Engine
	assigner module.ChunkAssigner // used to determine chunks this node needs to verify
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	verifier network.Engine,
	assigner module.ChunkAssigner,
) (*Engine, error) {
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log,
		me:       me,
		verifier: verifier,
		assigner: assigner,
	}

	_, err := net.Register(engine.ChunkDataPackProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register chunk data pack provider engine: %w", err)
	}
	return e, nil
}

// Ready initializes the engine and returns a channel that is closed when the initialization is done
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done terminates the engine and returns a channel that is closed when the termination is done
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

// process receives and submits an event to the engine for processing.
// It returns an error so the engine will not propagate an event unless
// it is successfully processed by the engine.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch resource := event.(type) {
	case *flow.ExecutionResult:
		return e.handleExecutionResult(resource)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) handleExecutionResult(result *flow.ExecutionResult) error {
	// receives an exec result (with 2 block references)
	// runs chunk assignment
	// for selected chunks sends requests for collections and chunk data packs
	// receives collections and chunk data packs
	// generates verifiable chunks and sends them to verifier engine.
	return nil
}

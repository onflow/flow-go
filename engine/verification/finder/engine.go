package finder

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
)

type Engine struct {
	unit  *engine.Unit
	log   zerolog.Logger
	me    module.Local
	match network.Engine
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	match network.Engine,
) (*Engine, error) {
	e := &Engine{
		unit:  engine.NewUnit(),
		log:   log,
		me:    me,
		match: match,
	}

	_, err := net.Register(engine.ExecutionReceiptProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution receipt provider channel: %w", err)
	}
	return e, nil
}

// Ready returns a channel that is closed when the verifier engine is ready.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a channel that is closed when the verifier engine is done.
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

// process receives and submits an event to the verifier engine for processing.
// It returns an error so the verifier engine will not propagate an event unless
// it is successfully processed by the engine.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch resource := event.(type) {
	case *flow.ExecutionReceipt:
		return e.handleExecutionReceipt(originID, resource)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) handleExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	// TODO: find the the block that include the guarantees of the collections
	// decides whether this exuection receipt is processable.
	// if processable, pass it to match engine
	return nil
}

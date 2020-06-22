package engine

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network"
)

type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	metrics  module.EngineMetrics
	me       module.Local
	con      network.Conduit
	handlers map[messages.Resource]HandlerFunc
}

// New creates a new consensus propagation engine.
func New(
	log zerolog.Logger, metrics module.EngineMetrics, net module.Network, me module.Local,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "synchronization").Logger(),
		metrics:  metrics,
		me:       me,
		handlers: make(map[messages.Resource]HandlerFunc),
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ProtocolProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
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
			e.log.Error().Err(err).Msg("synchronization could not process submitted event")
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

// Register will register a resource provider handler function with the engine.
func (e *Engine) Register(resource messages.Resource, handler HandlerFunc) error {

	_, exists := e.handlers[resource]
	if exists {
		return fmt.Errorf("resource handler already registered (%s)", resource)
	}

	e.handlers[resource] = handler
	return nil
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *messages.ResourceRequest:
		e.before(metrics.MessageResourceRequest)
		defer e.after(metrics.MessageResourceRequest)
		return e.onResourceRequest(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) before(msg string) {
	e.metrics.MessageReceived(metrics.EngineSynchronization, msg)
	e.unit.Lock()
}

func (e *Engine) after(msg string) {
	e.unit.Unlock()
	e.metrics.MessageHandled(metrics.EngineSynchronization, msg)
}

func (e *Engine) onResourceRequest(originID flow.Identifier, req *messages.ResourceRequest) error {

	handler, exists := e.handlers[req.Resource]
	if !exists {
		return fmt.Errorf("received request for unhandled resource type (%s)", req.Resource)
	}

	resource, err := handler(req.ResourceID)
	if err != nil {
		return fmt.Errorf("could not retrieve resource: %w", err)
	}

	rep := &messages.ResourceReply{
		Resource: req.Resource,
		Value:    resource,
		Nonce:    req.Nonce,
	}
	err = e.con.Submit(rep, originID)
	if err != nil {
		return fmt.Errorf("could not send resource response: %w", err)
	}

	return nil
}

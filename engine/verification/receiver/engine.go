// Package receiver implements engines for receiving resources from other
// node roles in the system.
package receiver

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
)

type Engine struct {
	unit        *engine.Unit
	log         zerolog.Logger
	con         network.Conduit
	me          module.Local
	state       protocol.State
	pool        verification.Mempool
	verifierEng network.Engine
}

// NewCollectionReceiver returns a new engine bound to the collection provider bus.
//
// This engine will only be able to receive, handle, and request resources from
// collection nodes.
func NewCollectionReceiver(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	state protocol.State,
	pool verification.Mempool,
	verifierEng network.Engine,
) (*Engine, error) {

	e := &Engine{
		unit:        engine.NewUnit(),
		log:         log.With().Str("engine", "receiver").Str("resource", "collection").Logger(),
		me:          me,
		state:       state,
		verifierEng: verifierEng,
		pool:        pool,
	}

	con, err := net.Register(engine.VerificationCollectionReceiver, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// NewExecutionReceiver returns a new engine bound to the execution provider bus.
//
// This engine will only be able to receive, handle, and request resources from
// execution nodes.
func NewExecutionReceiver(log zerolog.Logger, net module.Network, me module.Local, state protocol.State, pool verification.Mempool, verifierEng network.Engine) (*Engine, error) {

	e := &Engine{
		unit:        engine.NewUnit(),
		log:         log.With().Str("engine", "receiver").Str("resource", "execution").Logger(),
		me:          me,
		state:       state,
		verifierEng: verifierEng,
		pool:        pool,
	}

	con, err := net.Register(engine.VerificationExecutionReceiver, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
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

// RequestCollection submits a request for the given collection to collection nodes.
func (e *Engine) RequestCollection(hash flow.Fingerprint) error {
	collectionNodes, err := e.state.Final().Identities(identity.HasRole(flow.RoleCollection))
	if err != nil {
		return fmt.Errorf("could not get collection identities: %w", err)
	}

	req := &messages.CollectionRequest{Fingerprint: hash}

	err = e.con.Submit(req, collectionNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit collection request: %w", err)
	}

	return nil
}

// process processes events for the proposal engine on the collection node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.ExecutionReceipt:
		return e.onExecutionReceipt(originID, ev)
	case *messages.CollectionResponse:
		return e.onCollectionResponse(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) onExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {

	// TODO: add id of the ER once gets available
	e.log.Info().
		Hex("origin_id", originID[:]).
		Msg("execution receipt received")

	// TODO: correctness check for execution receipts

	// validate identity of the originID
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		// todo: potential attack on authenticity
		return errors.Errorf("invalid origin id %s", originID[:])
	}

	// validate that the sender is an execution node
	if id.Role != flow.Role(flow.RoleExecution) {
		// TODO: potential attack on integrity
		return errors.Errorf("invalid role for generating an execution receipt, id: %s, role: %s", id.NodeID, id.Role)
	}

	// store the execution receipt in the mempool
	isUnique := e.pool.Put(receipt)
	if !isUnique {
		return errors.New("received duplicate execution receipt")
	}

	// notify the verifier engine that we received an execution receipt
	e.verifierEng.SubmitLocal(receipt)

	return nil
}

func (e *Engine) onCollectionResponse(originID flow.Identifier, res *messages.CollectionResponse) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("collection_id", res.Fingerprint).
		Msg("collection received")

	coll := flow.Collection{Transactions: res.Transactions}

	// notify the verifier engine that we received a collection
	e.verifierEng.SubmitLocal(&coll)

	return nil
}

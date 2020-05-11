// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package propagation

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the propagation engine, which makes sure that new collections are
// propagated to the other consensus nodes on the network.
type Engine struct {
	unit       *engine.Unit            // used to control startup/shutdown
	log        zerolog.Logger          // used to log relevant actions with context
	metrics    module.EngineMetrics    // used to track sent & received messages
	mempool    module.MempoolMetrics   // used to track mempool sizes
	spans      module.ConsensusMetrics // used to track timespans
	con        network.Conduit         // used to talk to other nodes on the network
	state      protocol.State          // used to access the  protocol state
	me         module.Local            // used to access local node information
	guarantees mempool.Guarantees      // holds collection guarantees in memory
}

// New creates a new collection propagation engine.
func New(log zerolog.Logger, metrics module.EngineMetrics, mempool module.MempoolMetrics, spans module.ConsensusMetrics, net module.Network, state protocol.State, me module.Local, guarantees mempool.Guarantees) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:       engine.NewUnit(),
		log:        log.With().Str("engine", "propagation").Logger(),
		metrics:    metrics,
		mempool:    mempool,
		spans:      spans,
		state:      state,
		me:         me,
		guarantees: guarantees,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.BlockPropagation, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the propagation engine, it closes the channel when all submit goroutines
// have ended.
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

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.CollectionGuarantee:
		e.metrics.MessageReceived(metrics.EnginePropagation, metrics.MessageCollectionGuarantee)
		return e.onGuarantee(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onGuarantee is called when a new collection guarantee is received
// from another node on the network.
func (e *Engine) onGuarantee(originID flow.Identifier, guarantee *flow.CollectionGuarantee) error {

	log := e.log.With().
		Hex("origin_id", originID[:]).
		Hex("collection_id", logging.Entity(guarantee)).
		Int("signers", len(guarantee.SignerIDs)).
		Logger()

	log.Info().Msg("collection guarantee received")

	err := e.storeGuarantee(guarantee)
	if errors.Is(err, mempool.ErrAlreadyExists) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not store guarantee: %w", err)
	}

	log.Info().Msg("collection guarantee processed")

	// propagate the collection guarantee to other relevant nodes
	err = e.propagateGuarantee(guarantee)
	if err != nil {
		return fmt.Errorf("could not broadcast guarantee: %w", err)
	}

	log.Info().Msg("collection guarantee propagated to consensus nodes")

	return nil
}

// storeGuarantee will store a collection guarantee within the
// context of our local protocol state and memory pool.
func (e *Engine) storeGuarantee(guarantee *flow.CollectionGuarantee) error {

	// TODO: validate the collection guarantee signature

	// add the collection guarantee to our memory pool (also checks existence)
	err := e.guarantees.Add(guarantee)
	if err != nil {
		return fmt.Errorf("could not add guarantee to mempool: %w", err)
	}

	e.mempool.MempoolEntries(metrics.ResourceGuarantee, e.guarantees.Size())

	return nil
}

// propagateGuarantee will submit the collection guarantee to the
// network layer with all other consensus nodes as desired recipients.
func (e *Engine) propagateGuarantee(guarantee *flow.CollectionGuarantee) error {

	// select all the collection nodes on the network as our targets
	ids, err := e.state.Final().Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}

	// send the collection guarantee to all consensus identities
	targetIDs := ids.NodeIDs()
	err = e.con.Submit(guarantee, targetIDs...)
	if err != nil {
		return fmt.Errorf("could not send collection guarantee: %w", err)
	}

	e.metrics.MessageSent(metrics.EnginePropagation, metrics.MessageCollectionGuarantee)

	return nil
}

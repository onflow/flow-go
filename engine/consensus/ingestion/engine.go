// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

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
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine represents the ingestion engine, used to funnel collections from a
// cluster of collection nodes to the set of consensus nodes. It represents the
// link between collection nodes and consensus nodes and has a counterpart with
// the same engine ID in the collection node.
type Engine struct {
	unit    *engine.Unit            // used to manage concurrency & shutdown
	log     zerolog.Logger          // used to log relevant actions with context
	metrics module.EngineMetrics    // used to track sent & received messages
	tracer  module.Tracer           // used for tracing
	spans   module.ConsensusMetrics // used to track consensus spans
	mempool module.MempoolMetrics   // used to track mempool metrics
	state   protocol.State          // used to access the protocol state
	headers storage.Headers         // used to retrieve headers
	me      module.Local            // used to access local node information
	pool    mempool.Guarantees      // used to keep pending guarantees in pool
	con     network.Conduit         // conduit to receive/send guarantees
}

// New creates a new collection propagation engine.
func New(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	tracer module.Tracer,
	spans module.ConsensusMetrics,
	mempool module.MempoolMetrics,
	net module.Network,
	state protocol.State,
	headers storage.Headers,
	me module.Local,
	pool mempool.Guarantees,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:    engine.NewUnit(),
		log:     log.With().Str("engine", "ingestion").Logger(),
		metrics: metrics,
		mempool: mempool,
		tracer:  tracer,
		spans:   spans,
		state:   state,
		headers: headers,
		me:      me,
		pool:    pool,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.PushGuarantees, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the ingestion engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the ingestion engine, it only waits for all submit goroutines to end.
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
		err := e.process(originID, event)
		if engine.IsInvalidInputError(err) {
			e.log.Error().Str("error_type", "invalid_input").Err(err).Msg("ingestion received invalid event")
		} else if err != nil {
			e.log.Error().Err(err).Msg("ingestion could not process submitted event")
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

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.CollectionGuarantee:
		e.metrics.MessageReceived(metrics.EngineConsensusIngestion, metrics.MessageCollectionGuarantee)
		return e.onCollectionGuarantee(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onCollectionGuarantee is used to process collection guarantees received
// from nodes that are not consensus nodes (notably collection nodes).
func (e *Engine) onCollectionGuarantee(originID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	span := e.tracer.StartSpan(guarantee.CollectionID, trace.CONProcessCollection)
	// TODO finish span if we error? How are they shown in Jaeger?
	span.SetTag("collection_id", guarantee.CollectionID)
	childSpan := e.tracer.StartSpanFromParent(span, trace.CONIngOnCollectionGuarantee)
	defer childSpan.Finish()

	log := e.log.With().
		Hex("origin_id", originID[:]).
		Hex("collection_id", logging.Entity(guarantee)).
		Int("signers", len(guarantee.SignerIDs)).
		Logger()

	log.Info().Msg("collection guarantee received")

	// skip collection guarantees that are already in our memory pool
	exists := e.pool.Has(guarantee.ID())
	if exists {
		log.Debug().Msg("skipping known collection guarantee")
		return nil
	}

	// ensure there is at least one guarantor
	guarantors := guarantee.SignerIDs
	if len(guarantors) == 0 {
		return engine.NewInvalidInputError("invalid collection guarantee with no guarantors")
	}

	// get the identity of the origin node, so we can check if it's a valid
	// source for a collection guarantee (usually collection nodes)
	identity, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("could not get origin node identity: %w", err)
	}

	// we only accept guarantees from collection nodes or other consensus nodes
	if identity.Role != flow.RoleCollection && identity.Role != flow.RoleConsensus {
		return fmt.Errorf("invalid origin role for guarantee (%s)", identity.Role)
	}

	// ensure that collection has not expired
	err = e.validateExpiry(guarantee)
	if err != nil {
		return fmt.Errorf("could not validate guarantee expiry: %w", err)
	}

	// get the clusters to assign the guarantee and check if the guarantor is part of it
	clusters, err := e.state.Final().Clusters()
	if err != nil {
		return fmt.Errorf("could not get clusters: %w", err)
	}
	cluster, ok := clusters.ByNodeID(guarantors[0])
	if !ok {
		return engine.NewInvalidInputErrorf("guarantor (id=%s) does not exist in any cluster", guarantors[0])
	}

	// NOTE: Eventually we should check the signatures, ensure a quorum of the
	// cluster, and ensure HotStuff finalization rules. Likely a cluster-specific
	// version of the follower will be a good fit for this. For now, collection
	// nodes independently decide when a collection is finalized and we only check
	// that the guarantors are all from the same cluster.

	// ensure the guarantors are from the same cluster
	for _, guarantorID := range guarantors {
		_, exists := cluster.ByNodeID(guarantorID)
		if !exists {
			return engine.NewInvalidInputError("inconsistent guarantors from different clusters")
		}
	}

	// at this point, we can add the guarantee to the memory pool
	added := e.pool.Add(guarantee)
	if !added {
		log.Debug().Msg("discarding guarantee already in pool")
		return nil
	}

	e.mempool.MempoolEntries(metrics.ResourceGuarantee, e.pool.Size())

	log.Info().Msg("collection guarantee added to pool")

	// if the collection guarantee stems from a collection node, we should propagate it
	// to the other consensus nodes on the network; we no longer need to care about
	// fan-out here, as libp2p will take care of the pub-sub pattern adequately
	if identity.Role != flow.RoleCollection {
		return nil
	}

	// NOTE: there are two ways to go about this:
	// - expect the collection nodes to propagate the guarantee to all consensus nodes;
	// - ensure that we take care of propagating guarantees to other consensus nodes.
	// It's probably a better idea to make sure the consensus nodes take care of this.
	// The consensus committee is the backbone of the network and should rely as little
	// as possible on correct behaviour from other node roles. At the same time, there
	// are likely to be significantly more consensus nodes on the network, which means
	// it's a better usage of resources to distribute the load for propagation over
	// consensus node committee than over the collection clusters.

	// select all the consensus nodes on the network as our targets
	identities, err := e.state.Final().Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get committee identities: %w", err)
	}

	// send the collection guarantee to all consensus identities
	err = e.con.Submit(guarantee, identities.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not send collection guarantee: %w", err)
	}

	log.Info().Msg("collection guarantee broadcasted to committee")

	return nil
}

func (e *Engine) validateExpiry(guarantee *flow.CollectionGuarantee) error {

	// get the last finalized header and the reference block header
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized header: %w", err)
	}
	ref, err := e.headers.ByBlockID(guarantee.ReferenceBlockID)
	if errors.Is(err, storage.ErrNotFound) {
		return engine.NewInvalidInputErrorf("collection guarantee refers to an unknown block: %x", guarantee.ReferenceBlockID)
	}

	// if head has advanced beyond the block referenced by the collection guarantee by more than 'expiry' number of blocks,
	// then reject the collection
	diff := final.Height - ref.Height
	// check for overflow
	if diff > final.Height {
		diff = 0
	}
	if diff > flow.DefaultTransactionExpiry {
		return engine.NewInvalidInputErrorf("collection guarantee expired ref_height=%d final_height=%d", ref.Height, final.Height)
	}

	return nil
}

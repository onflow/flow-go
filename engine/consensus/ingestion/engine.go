// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
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
	spans   module.ConsensusMetrics // used to track consensus spans
	prop    network.Engine          // used to process & propagate collections
	state   protocol.State          // used to access the  protocol state
	me      module.Local            // used to access local node information
	// the number of blocks that can be between the reference block and the
	// finalized head before we consider the transaction expired
	expiry uint64
}

// New creates a new collection propagation engine.
func New(log zerolog.Logger, metrics module.EngineMetrics, net module.Network, prop network.Engine, state protocol.State, me module.Local) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:    engine.NewUnit(),
		log:     log.With().Str("engine", "ingestion").Logger(),
		metrics: metrics,
		prop:    prop,
		state:   state,
		me:      me,
		expiry:  flow.DefaultTransactionExpiry,
	}

	// register the engine with the network layer and store the conduit
	_, err := net.Register(engine.CollectionProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

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

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.CollectionGuarantee:
		e.metrics.MessageReceived(metrics.EngineIngestion, metrics.MessageCollectionGuarantee)
		return e.onCollectionGuarantee(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onCollectionGuarantee is used to process collection guarantees received
// from nodes that are not consensus nodes (notably collection nodes).
func (e *Engine) onCollectionGuarantee(originID flow.Identifier, guarantee *flow.CollectionGuarantee) error {

	log := e.log.With().
		Hex("origin_id", originID[:]).
		Hex("collection_id", logging.Entity(guarantee)).
		Int("signers", len(guarantee.SignerIDs)).
		Logger()

	log.Info().Msg("collection guarantee received from collection cluster")

	final := e.state.Final()

	guarantors := guarantee.SignerIDs

	// ensure there is at least one guarantor
	if len(guarantors) == 0 {
		return fmt.Errorf("invalid collection guarantee with no guarantors")
	}

	// get the identity of the origin node, so we can check if it's a valid
	// source for a collection guarantee (usually collection nodes)
	id, err := final.Identity(originID)
	if err != nil {
		return fmt.Errorf("could not get origin node identity: %w", err)
	}

	// check that the origin is a collection node; this check is fine even if it
	// excludes our own ID - in the case of local submission of collections, we
	// should use the propagation engine, which is for exchange of collections
	// between consensus nodes anyway; we do no processing or validation in this
	// engine beyond validating the origin
	if id.Role != flow.RoleCollection {
		return fmt.Errorf("invalid origin node role (%s)", id.Role)
	}

	// ensure that collection has not expired
	err = e.validateCollectionGuaranteeExpiry(guarantee, final)
	if err != nil {
		return err
	}

	clusters, err := final.Clusters()
	if err != nil {
		return fmt.Errorf("could not get clusters: %w", err)
	}

	cluster, ok := clusters.ByNodeID(guarantors[0])
	if !ok {
		return fmt.Errorf("guarantor (id=%s) does not exist in any cluster", guarantors[0])
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
			return fmt.Errorf("inconsistent guarantors from different clusters")
		}
	}

	log.Info().Msg("forwarding collection guarantee to propagation engine")

	// submit the collection to the propagation engine - this is non-blocking
	// we could just validate it here and add it to the memory pool directly,
	// but then we would duplicate the validation logic
	e.prop.SubmitLocal(guarantee)

	return nil
}

func (e *Engine) validateCollectionGuaranteeExpiry(guarantee *flow.CollectionGuarantee, snapshot protocol.Snapshot) error {

	head, err := snapshot.Head()
	if err != nil {
		return fmt.Errorf("could not get finalized header: %w", err)
	}

	ref, err := e.state.AtBlockID(guarantee.ReferenceBlockID).Head()
	if errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("collection guarantee refers to an unknown block: %x", guarantee.ReferenceBlockID)
	}

	// if head has advanced beyond the block referenced by the collection guarantee by more than 'expiry' number of blocks,
	// then reject the collection
	diff := head.Height - ref.Height
	// check for overflow
	if ref.Height > head.Height {
		diff = 0
	}
	if diff > e.expiry {
		return fmt.Errorf("collection guarantee expired ref_height=%d final_height=%d", ref.Height, head.Height)
	}

	return nil
}

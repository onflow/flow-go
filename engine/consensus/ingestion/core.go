// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Core represents the ingestion engine, used to funnel collections from a
// cluster of collection nodes to the set of consensus nodes. It represents the
// link between collection nodes and consensus nodes and has a counterpart with
// the same engine ID in the collection node.
type Core struct {
	log     zerolog.Logger        // used to log relevant actions with context
	tracer  module.Tracer         // used for tracing
	mempool module.MempoolMetrics // used to track mempool metrics
	state   protocol.State        // used to access the protocol state
	headers storage.Headers       // used to retrieve headers
	me      module.Local          // used to access local node information
	pool    mempool.Guarantees    // used to keep pending guarantees in pool
}

// New creates a new collection propagation engine.
func NewCore(
	log zerolog.Logger,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	state protocol.State,
	headers storage.Headers,
	me module.Local,
	pool mempool.Guarantees,
) (*Core, error) {

	// initialize the propagation engine with its dependencies
	e := &Core{
		log:     log.With().Str("ingestion", "core").Logger(),
		tracer:  tracer,
		mempool: mempool,
		state:   state,
		headers: headers,
		me:      me,
		pool:    pool,
	}

	return e, nil
}

// OnGuarantee is used to process collection guarantees received
// from nodes that are not consensus nodes (notably collection nodes).
// Returns expected errors:
// * InvalidInputError
// * UnverifiableInputError
// * OutdatedInputError
func (e *Core) OnGuarantee(originID flow.Identifier, guarantee *flow.CollectionGuarantee) error {

	span, _, isSampled := e.tracer.StartCollectionSpan(context.Background(), guarantee.CollectionID, trace.CONIngOnCollectionGuarantee)
	if isSampled {
		span.LogKV("originID", originID.String())
	}
	defer span.Finish()

	guaranteeID := guarantee.ID()

	log := e.log.With().
		Hex("origin_id", originID[:]).
		Hex("collection_id", guaranteeID[:]).
		Int("signers", len(guarantee.SignerIDs)).
		Logger()

	log.Info().Msg("collection guarantee received")

	// skip collection guarantees that are already in our memory pool
	exists := e.pool.Has(guaranteeID)
	if exists {
		log.Debug().Msg("skipping known collection guarantee")
		return nil
	}

	// retrieve and validate the sender of the guarantee message
	origin, err := e.validateOrigin(originID, guarantee)
	if err != nil {
		return fmt.Errorf("origin validation error: %w", err)
	}

	// ensure that collection has not expired
	err = e.validateExpiry(guarantee)
	if err != nil {
		return fmt.Errorf("expiry validation error: %w", err)
	}

	// ensure the guarantors are allowed to produce this collection
	err = e.validateGuarantors(guarantee)
	if err != nil {
		return fmt.Errorf("guarantor validation error: %w", err)
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
	if origin.Role != flow.RoleCollection {
		return nil
	}

	final := e.state.Final()

	// don't propagate collection guarantees if we are not currently staked
	staked, err := protocol.IsNodeStakedAt(final, e.me.NodeID())
	if err != nil {
		return fmt.Errorf("could not check my staked status: %w", err)
	}
	if !staked {
		return nil
	}

	// NOTE: there are two ways to go about this:
	// - expect the collection nodes to propagate the guarantee to all consensus nodes;
	// - ensure that we take care of propagating guarantees to other consensus nodes.
	// Currently, we go with first option as each collection node broadcasts a guarantee to
	// all consensus nodes. So we expect all collections of a cluster to broadcast a guarantee to
	// all consensus nodes. Even on an unhappy path, as long as only one collection node does it
	// the guarantee must be delivered to all consensus nodes.

	return nil
}

// validateExpiry validates that the collection has not expired w.r.t. the local
// latest finalized block.
func (e *Core) validateExpiry(guarantee *flow.CollectionGuarantee) error {

	// get the last finalized header and the reference block header
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized header: %w", err)
	}
	ref, err := e.headers.ByBlockID(guarantee.ReferenceBlockID)
	if errors.Is(err, storage.ErrNotFound) {
		return engine.NewUnverifiableInputError("collection guarantee refers to an unknown block (id=%x): %w", guarantee.ReferenceBlockID, err)
	}

	// if head has advanced beyond the block referenced by the collection guarantee by more than 'expiry' number of blocks,
	// then reject the collection
	if ref.Height > final.Height {
		return nil // the reference block is newer than the latest finalized one
	}
	if final.Height-ref.Height > flow.DefaultTransactionExpiry {
		return engine.NewOutdatedInputErrorf("collection guarantee expired ref_height=%d final_height=%d", ref.Height, final.Height)
	}

	return nil
}

// validateGuarantors validates that the guarantors of a collection are valid,
// in that they are all from the same cluster and that cluster is allowed to
// produce the given collection w.r.t. the guarantee's reference block.
func (e *Core) validateGuarantors(guarantee *flow.CollectionGuarantee) error {

	guarantors := guarantee.SignerIDs

	if len(guarantors) == 0 {
		return engine.NewInvalidInputError("invalid collection guarantee with no guarantors")
	}

	// get the clusters to assign the guarantee and check if the guarantor is part of it
	snapshot := e.state.AtBlockID(guarantee.ReferenceBlockID)
	clusters, err := snapshot.Epochs().Current().Clustering()
	if errors.Is(err, storage.ErrNotFound) {
		return engine.NewUnverifiableInputError("could not get clusters for unknown reference block (id=%x): %w", guarantee.ReferenceBlockID, err)
	}
	if err != nil {
		return fmt.Errorf("internal error retrieving collector clusters: %w", err)
	}
	cluster, _, ok := clusters.ByNodeID(guarantors[0])
	if !ok {
		return engine.NewInvalidInputErrorf("guarantor (id=%s) does not exist in any cluster", guarantors[0])
	}

	// NOTE: Eventually we should check the signatures, ensure a quorum of the
	// cluster, and ensure HotStuff finalization rules. Likely a cluster-specific
	// version of the follower will be a good fit for this. For now, collection
	// nodes independently decide when a collection is finalized and we only check
	// that the guarantors are all from the same cluster.

	// ensure the guarantors are from the same cluster
	clusterLookup := cluster.Lookup()
	for _, guarantorID := range guarantors {
		_, exists := clusterLookup[guarantorID]
		if !exists {
			return engine.NewInvalidInputError("inconsistent guarantors from different clusters")
		}
	}

	return nil
}

// validateOrigin validates that the message has a valid sender (origin). We
// allow Guarantee messages either from collection nodes or from consensus nodes.
// Collection nodes must be valid in the identity table w.r.t. the reference
// block of the collection; consensus nodes must be valid w.r.t. the latest
// finalized block.
//
// Returns the origin identity as validated if validation passes.
func (e *Core) validateOrigin(originID flow.Identifier, guarantee *flow.CollectionGuarantee) (*flow.Identity, error) {

	refBlock := guarantee.ReferenceBlockID

	// get the origin identity w.r.t. the collection reference block
	origin, err := e.state.AtBlockID(refBlock).Identity(originID)
	if err != nil {
		// collection with an unknown reference block is unverifiable
		if errors.Is(err, storage.ErrNotFound) {
			return nil, engine.NewUnverifiableInputError("could not get origin (id=%x) for unknown reference block (id=%x): %w", originID, refBlock, err)
		}
		// in case of identity not found, we continue and check the other case - all other errors are unexpected
		if !protocol.IsIdentityNotFound(err) {
			return nil, fmt.Errorf("could not get origin (id=%x) identity w.r.t. reference block (id=%x) : %w", originID, refBlock, err)
		}
	} else {
		// if it is a collection node and a valid epoch participant, it is a valid origin
		if filter.And(
			filter.IsValidCurrentEpochParticipant,
			filter.HasRole(flow.RoleCollection),
		)(origin) {
			return origin, nil
		}
	}

	// At this point there are two cases:
	// 1) origin was not found at reference block state
	// 2) origin was found at reference block state, but isn't a collection node
	//
	// Now, check whether the origin is a valid consensus node using finalized state.
	// This can happen when we have just passed an epoch boundary and a consensus
	// node which has just joined sends us a guarantee referencing a block prior
	// to the epoch boundary.

	// get the origin identity w.r.t. the latest finalized block
	origin, err = e.state.Final().Identity(originID)
	if protocol.IsIdentityNotFound(err) {
		return nil, engine.NewInvalidInputErrorf("unknown origin (id=%x): %w", originID, err)
	}
	if err != nil {
		return nil, fmt.Errorf("could not get origin (id=%x) w.r.t. finalized state: %w", originID, err)
	}

	if filter.And(
		filter.IsValidCurrentEpochParticipant,
		filter.HasRole(flow.RoleConsensus),
	)(origin) {
		return origin, nil
	}

	return nil, engine.NewInvalidInputErrorf("invalid origin (id=%x)", originID)
}

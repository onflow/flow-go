// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Core represents core logic of the ingestion engine. It contains logic
// for handling single collection which are channeled from engine in concurrent way.
type Core struct {
	log     zerolog.Logger        // used to log relevant actions with context
	tracer  module.Tracer         // used for tracing
	mempool module.MempoolMetrics // used to track mempool metrics
	state   protocol.State        // used to access the protocol state
	headers storage.Headers       // used to retrieve headers
	pool    mempool.Guarantees    // used to keep pending guarantees in pool
}

func NewCore(
	log zerolog.Logger,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	state protocol.State,
	headers storage.Headers,
	pool mempool.Guarantees,
) *Core {
	return &Core{
		log:     log.With().Str("ingestion", "core").Logger(),
		tracer:  tracer,
		mempool: mempool,
		state:   state,
		headers: headers,
		pool:    pool,
	}
}

// OnGuarantee is used to process collection guarantees received
// from nodes that are not consensus nodes (notably collection nodes).
// Returns expected errors:
//  * engine.InvalidInputError if the collection violates protocol rules
//  * engine.UnverifiableInputError if the reference block of the collection is unknown
//  * engine.OutdatedInputError if the collection is already expired
// All other errors are unexpected and potential symptoms of internal state corruption.
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

	// check collection guarantee's validity
	err := e.validateOrigin(originID, guarantee) // retrieve and validate the sender of the collection guarantee
	if err != nil {
		return fmt.Errorf("origin validation error: %w", err)
	}
	err = e.validateExpiry(guarantee) // ensure that collection has not expired
	if err != nil {
		return fmt.Errorf("expiry validation error: %w", err)
	}
	err = e.validateGuarantors(guarantee) // ensure the guarantors are allowed to produce this collection
	if err != nil {
		return fmt.Errorf("guarantor validation error: %w", err)
	}

	// at this point, we can add the guarantee to the memory pool
	added := e.pool.Add(guarantee)
	if !added {
		log.Debug().Msg("discarding guarantee already in pool")
		return nil
	}
	log.Info().Msg("collection guarantee added to pool")

	e.mempool.MempoolEntries(metrics.ResourceGuarantee, e.pool.Size())
	return nil
}

// validateExpiry validates that the collection has not expired w.r.t. the local
// latest finalized block.
// Expected errors during normal operation:
//  * engine.UnverifiableInputError if the reference block of the collection is unknown
//  * engine.OutdatedInputError if the collection is already expired
// All other errors are unexpected and potential symptoms of internal state corruption.
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
// Expected errors during normal operation:
//  * engine.InvalidInputError if the origin violates any requirements
//  * engine.UnverifiableInputError if the reference block of the collection is unknown
// All other errors are unexpected and potential symptoms of internal state corruption.
// TODO: Eventually we should check the signatures, ensure a quorum of the
//	     cluster, and ensure HotStuff finalization rules. Likely a cluster-specific
//	     version of the follower will be a good fit for this. For now, collection
//	     nodes independently decide when a collection is finalized and we only check
//	     that the guarantors are all from the same cluster. This implementation is NOT BFT.
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
// only accept guarantees from an origin that is part of the identity table
// at the collection's reference block. Furthermore, the origin must be
// an authorized (i.e. positive weight), non-ejected collector node.
// Expected errors during normal operation:
//  * engine.InvalidInputError if the origin violates any requirements
//  * engine.UnverifiableInputError if the reference block of the collection is unknown
// All other errors are unexpected and potential symptoms of internal state corruption.
//
// TODO: ultimately, the origin broadcasting a collection is irrelevant, as long as the
//       collection itself is valid. The origin is only needed in case the guarantee is found
//       to be invalid, in which case we might want to slash the origin.
func (e *Core) validateOrigin(originID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	refState := e.state.AtBlockID(guarantee.ReferenceBlockID)
	valid, err := protocol.IsNodeAuthorizedWithRoleAt(refState, originID, flow.RoleCollection)
	if err != nil {
		// collection with an unknown reference block is unverifiable
		if errors.Is(err, storage.ErrNotFound) {
			return engine.NewUnverifiableInputError("could not get origin (id=%x) for unknown reference block (id=%x): %w", originID, guarantee.ReferenceBlockID, err)
		}
		return fmt.Errorf("unexpected error checking collection origin %x at reference block %x: %w", originID, guarantee.ReferenceBlockID, err)
	}
	if !valid {
		return engine.NewInvalidInputErrorf("invalid collection origin (id=%x)", originID)
	}
	return nil
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// Builder is the builder for consensus block payloads. Upon providing a payload
// hash, it also memorizes which entities were included into the payload.
type Builder struct {
	metrics  module.MempoolMetrics
	tracer   module.Tracer
	db       *badger.DB
	state    protocol.State
	seals    storage.Seals
	headers  storage.Headers
	index    storage.Index
	guarPool mempool.Guarantees
	sealPool mempool.IncorporatedResultSeals
	cfg      Config
}

// NewBuilder creates a new block builder.
func NewBuilder(
	metrics module.MempoolMetrics,
	db *badger.DB,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	index storage.Index,
	guarPool mempool.Guarantees,
	sealPool mempool.IncorporatedResultSeals,
	tracer module.Tracer,
	options ...func(*Config),
) *Builder {

	// initialize default config
	cfg := Config{
		minInterval:  500 * time.Millisecond,
		maxInterval:  10 * time.Second,
		maxSealCount: 100,
		expiry:       flow.DefaultTransactionExpiry,
	}

	// apply option parameters
	for _, option := range options {
		option(&cfg)
	}

	b := &Builder{
		metrics:  metrics,
		db:       db,
		tracer:   tracer,
		state:    state,
		headers:  headers,
		seals:    seals,
		index:    index,
		guarPool: guarPool,
		sealPool: sealPool,
		cfg:      cfg,
	}
	return b
}

// BuildOn creates a new block header build on the provided parent, using the given view and applying the
// custom setter function to allow the caller to make changes to the header before storing it.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {

	b.tracer.StartSpan(parentID, trace.CONBuildOn)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOn)

	// STEP ONE: Create a lookup of all previously used guarantees on the part
	// of the chain that we are building on. We do this separately for pending
	// and finalized ancestors, so we can differentiate what to do about it.

	b.tracer.StartSpan(parentID, trace.CONBuildOnSetup)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnSetup)

	var finalized uint64
	err := b.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = b.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return nil, fmt.Errorf("could not lookup finalized block: %w", err)
	}

	b.tracer.FinishSpan(parentID, trace.CONBuildOnSetup)
	b.tracer.StartSpan(parentID, trace.CONBuildOnUnfinalizedLookup)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnUnfinalizedLookup)

	ancestorID := parentID
	pendingLookup := make(map[flow.Identifier]struct{})
	for ancestorID != finalID {
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		if ancestor.Height <= finalized {
			return nil, fmt.Errorf("should always build on last finalized block")
		}
		index, err := b.index.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}
		for _, collID := range index.CollectionIDs {
			pendingLookup[collID] = struct{}{}
		}
		ancestorID = ancestor.ParentID
	}

	b.tracer.FinishSpan(parentID, trace.CONBuildOnUnfinalizedLookup)
	b.tracer.StartSpan(parentID, trace.CONBuildOnFinalizedLookup)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnFinalizedLookup)

	// we look back only as far as the expiry limit for the current height we
	// are building for; any guarantee with a reference block before that can
	// not be included anymore anyway
	parent, err := b.headers.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent: %w", err)
	}
	height := parent.Height + 1
	limit := height - uint64(b.cfg.expiry)
	if limit > height { // overflow check
		limit = 0
	}

	// look up the root height so we don't look too far back
	// initially this is the genesis block height (aka 0).
	var rootHeight uint64
	err = b.db.View(operation.RetrieveRootHeight(&rootHeight))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root block height: %w", err)
	}
	if limit < rootHeight {
		limit = rootHeight
	}

	ancestorID = finalID
	finalLookup := make(map[flow.Identifier]struct{})
	for {
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		index, err := b.index.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}
		for _, collID := range index.CollectionIDs {
			finalLookup[collID] = struct{}{}
		}
		if ancestor.Height <= limit {
			break
		}
		ancestorID = ancestor.ParentID
	}

	b.tracer.FinishSpan(parentID, trace.CONBuildOnFinalizedLookup)
	b.tracer.StartSpan(parentID, trace.CONBuildOnCreatePayloadGuarantees)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnCreatePayloadGuarantees)

	// STEP TWO: Go through the guarantees in our memory pool.
	// 1) If it was already included on the finalized part of the chain, remove
	// it from the memory pool and skip.
	// 2) If the reference block has an expired height, also remove it from the
	// memory pool and skip.
	// 3) If it was already included on the pending part of the chain, skip, but
	// keep in memory pool for now.
	// 4) Otherwise, this guarantee can be included in the payload.

	var guarantees []*flow.CollectionGuarantee
	for _, guarantee := range b.guarPool.All() {
		collID := guarantee.ID()
		_, duplicated := finalLookup[collID]
		if duplicated {
			_ = b.guarPool.Rem(collID)
			continue
		}
		ref, err := b.headers.ByBlockID(guarantee.ReferenceBlockID)
		if errors.Is(err, storage.ErrNotFound) {
			_ = b.guarPool.Rem(collID)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get reference block: %w", err)
		}
		if ref.Height < limit {
			_ = b.guarPool.Rem(collID)
			continue
		}
		_, duplicated = pendingLookup[collID]
		if duplicated {
			continue
		}
		guarantees = append(guarantees, guarantee)
	}

	b.metrics.MempoolEntries(metrics.ResourceGuarantee, b.guarPool.Size())

	b.tracer.FinishSpan(parentID, trace.CONBuildOnCreatePayloadGuarantees)
	b.tracer.StartSpan(parentID, trace.CONBuildOnCreatePayloadSeals)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnCreatePayloadSeals)

	// STEP FOUR: We try to get all ancestors from last sealed block all the way
	// to the parent. Then we try to get seals for each of them until we don't
	// find one. This creates a valid chain of seals from the last sealed block
	// to at most the parent.

	// TODO: The following logic for selecting seals will be replaced to match
	// seals to incorporated results, looping through the fork, from parent to
	// last sealed, to inspect incorporated receipts and check if the mempool
	// contains corresponding seals.
	// This will be implemented in phase 2 of the verification and sealing
	// roadmap (https://github.com/dapperlabs/flow-go/issues/4872)

	// create a mapping of block to seal for all seals in our pool
	byBlock, err := b.block2SealMap()
	if err != nil {
		return nil, fmt.Errorf("failed to construct map from blockID to seal: %w", err)
	}

	// get the parent's block seal, which constitutes the beginning of the
	// sealing chain; this is where we need to start with our chain of seals
	last, err := b.seals.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", parentID, err)
	}

	// get the last sealed block; we use its height to iterate forwards through
	// the finalized blocks which still need sealing
	sealed, err := b.headers.ByBlockID(last.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve sealed block (%x): %w", last.BlockID, err)
	}

	// we now go from last sealed height plus one to finalized height and check
	// if we have the seal for each of them step by step; often we will not even
	// enter this loop, because last sealed height is higher than finalized
	unchained := false
	var seals []*flow.Seal
	var sealCount uint
	for height := sealed.Height + 1; height <= finalized; height++ {
		if len(byBlock) == 0 {
			break
		}

		// add at most <maxSealCount> number of seals in a new block proposal
		// in order to prevent the block payload from being too big.
		if sealCount >= b.cfg.maxSealCount {
			break
		}

		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get block for height (%d): %w", height, err)
		}
		blockID := header.ID()
		next, found := byBlock[blockID]
		if !found {
			unchained = true
			break
		}

		// enforce that execution results form chain
		nextResultToBeSealed := next.IncorporatedResult.Result
		initialState, ok := nextResultToBeSealed.InitialStateCommit()
		if !ok {
			return nil, fmt.Errorf("missing initial state commitment in execution result %v", nextResultToBeSealed.ID())
		}
		if !bytes.Equal(initialState, last.FinalState) {
			return nil, fmt.Errorf("execution results do not form chain")
		}

		seals = append(seals, next.Seal)
		sealCount++
		delete(byBlock, blockID)
		last = next.Seal
	}

	// NOTE: We should only run the next part in case we did not use up all
	// seals in the previous part; both break cases should make us skip the rest
	// as it means we either ran out of seals or we can't find the next link in
	// the chain.

	// Once we have filled in seals for all finalized blocks we need to check
	// the non-finalized blocks backwards; collect all of them, from direct
	// parent to just before finalized, and see if we can use up the rest of the
	// seals. We need to be careful to break when reaching the last sealed block
	// as it could be higher than the last finalized block.
	ancestorID = parentID
	var pendingIDs []flow.Identifier
	for ancestorID != finalID && ancestorID != last.BlockID {
		pendingIDs = append(pendingIDs, ancestorID)
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get sealable ancestor (%x): %w", ancestorID, err)
		}
		ancestorID = ancestor.ParentID
	}
	for i := len(pendingIDs) - 1; i >= 0; i-- {
		if len(byBlock) == 0 {
			break
		}
		// add at most <maxSealCount> number of seals in a new block proposal
		// in order to prevent the block payload from being too big.
		if sealCount >= b.cfg.maxSealCount {
			break
		}
		if unchained {
			break
		}
		pendingID := pendingIDs[i]
		next, found := byBlock[pendingID]
		if !found {
			break
		}

		seals = append(seals, next.Seal)
		sealCount++
		delete(byBlock, pendingID)
	}

	b.tracer.FinishSpan(parentID, trace.CONBuildOnCreatePayloadSeals)
	b.tracer.StartSpan(parentID, trace.CONBuildOnCreateHeader)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnCreateHeader)

	// STEP FOUR: We now have guarantees and seals we can validly include
	// in the payload built on top of the given parent. Now we need to build
	// and store the block header, as well as index the payload contents.

	// build the payload so we can get the hash
	payload := &flow.Payload{
		Guarantees: guarantees,
		Seals:      seals,
	}

	// calculate the timestamp and cutoffs
	timestamp := time.Now().UTC()
	from := parent.Timestamp.Add(b.cfg.minInterval)
	to := parent.Timestamp.Add(b.cfg.maxInterval)

	// adjust timestamp if outside of cutoffs
	if timestamp.Before(from) {
		timestamp = from
	}
	if timestamp.After(to) {
		timestamp = to
	}

	// construct default block on top of the provided parent
	header := &flow.Header{
		ChainID:     parent.ChainID,
		ParentID:    parentID,
		Height:      height,
		Timestamp:   timestamp,
		PayloadHash: payload.Hash(),

		// the following fields should be set by the custom function as needed
		// NOTE: we could abstract all of this away into an interface{} field,
		// but that would be over the top as we will probably always use hotstuff
		View:           0,
		ParentVoterIDs: nil,
		ParentVoterSig: nil,
		ProposerID:     flow.ZeroID,
		ProposerSig:    nil,
	}

	// apply the custom fields setter of the consensus algorithm
	err = setter(header)
	if err != nil {
		return nil, fmt.Errorf("could not apply setter: %w", err)
	}

	// insert the proposal into the database
	proposal := &flow.Block{
		Header:  header,
		Payload: payload,
	}

	b.tracer.FinishSpan(parentID, trace.CONBuildOnCreateHeader)
	b.tracer.StartSpan(parentID, trace.CONBuildOnDBInsert)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnDBInsert)

	err = b.state.Mutate().Extend(proposal)
	if err != nil {
		return nil, fmt.Errorf("could not extend state with built proposal: %w", err)
	}

	return header, nil
}

// block2SealMap creates a map: blockID -> seal
// from all seals in our mempool. Its also checks for inconsistent seals:
// we consider two seals as inconsistent, if they have different start or end states
func (b *Builder) block2SealMap() (map[flow.Identifier]*flow.IncorporatedResultSeal, error) {
	// TODO: the following implementation is somewhat temporary and contains a lot of sanity checks
	//       probably should be cleaned up once we have full sealing
	encounteredInconsistentSealsForSameBlock := false
	byBlock := make(map[flow.Identifier]*flow.IncorporatedResultSeal)
	for _, irSeal := range b.sealPool.All() {
		irSealInitialState, ok := irSeal.IncorporatedResult.Result.InitialStateCommit()
		if !ok || len(irSealInitialState) < 1 { // missing or empty initial state commitment
			// fatal error: respective Execution Result should have been rejected by matching engine
			return nil, fmt.Errorf("seal for result without initial state: %v", irSeal.IncorporatedResult.Result.ID())
		}
		irSealFinalState, ok := irSeal.IncorporatedResult.Result.FinalStateCommitment()
		if !ok || len(irSealFinalState) < 1 { // missing or empty final state commitment
			// fatal error: respective Execution Result should have been rejected by matching engine
			return nil, fmt.Errorf("seal for result without final state: %v", irSeal.IncorporatedResult.Result.ID())
		}
		if !bytes.Equal(irSeal.Seal.FinalState, irSealFinalState) {
			// fatal error: matching engine has constructed seal with inconsistent values
			return nil, fmt.Errorf("inconsistent final sate in result and seal for block %v", irSeal.Seal.BlockID)
		}
		// all Seals that are added to map have initial and final state commitment (otherwise, they are rejected by code above)

		if irSeal2, found := byBlock[irSeal.Seal.BlockID]; found {
			sc1json, err := json.Marshal(irSeal)
			if err != nil {
				return nil, err
			}
			sc2json, err := json.Marshal(irSeal2)
			if err != nil {
				return nil, err
			}
			block, err := b.headers.ByBlockID(irSeal.Seal.BlockID)
			if err != nil {
				// not finding the block for a seal is a fatal, internal error: respective Execution Result should have been rejected by matching engine
				// we still print as much of the error message as we can about the inconsistent seals
				fmt.Printf("WARNING: multiple seals with different IDs for the same block %v: %s and %s\n", irSeal.Seal.BlockID, string(sc1json), string(sc2json))
				return nil, fmt.Errorf("could not retrieve block for seal: %w", err)
			}

			// by induction, all elements in byBlock should have initial and final state commitment
			irSeal2InitialState, _ := irSeal2.IncorporatedResult.Result.InitialStateCommit()
			irSeal2FinalState, _ := irSeal2.IncorporatedResult.Result.FinalStateCommitment()

			// check whether seals are inconsistent:
			if !bytes.Equal(irSealFinalState, irSeal2FinalState) || !bytes.Equal(irSealInitialState, irSeal2InitialState) {
				fmt.Printf("ERROR: inconsistent seals for the same block %v at height %d: %s and %s\n", irSeal.Seal.BlockID, block.Height, string(sc1json), string(sc2json))
				encounteredInconsistentSealsForSameBlock = true
			} else {
				fmt.Printf("WARNING: multiple seals with different IDs for the same block %v at height %d: %s and %s\n", irSeal.Seal.BlockID, block.Height, string(sc1json), string(sc2json))
			}

		} else {
			byBlock[irSeal.Seal.BlockID] = irSeal
		}
	}
	if encounteredInconsistentSealsForSameBlock {
		// in case we find inconsistent seals, do not seal anything
		byBlock = make(map[flow.Identifier]*flow.IncorporatedResultSeal)
	}
	return byBlock, nil
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"errors"
	"fmt"
	"sort"
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
	blocks   storage.Blocks
	guarPool mempool.Guarantees
	sealPool mempool.IncorporatedResultSeals
	recPool  mempool.Receipts
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
	blocks storage.Blocks,
	guarPool mempool.Guarantees,
	sealPool mempool.IncorporatedResultSeals,
	recPool mempool.Receipts,
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
		blocks:   blocks,
		guarPool: guarPool,
		sealPool: sealPool,
		recPool:  recPool,
		cfg:      cfg,
	}
	return b
}

// BuildOn creates a new block header on top of the provided parent, using the
// given view and applying the custom setter function to allow the caller to
// make changes to the header before storing it.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {

	b.tracer.StartSpan(parentID, trace.CONBuildOn)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOn)

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

	// get the collection guarantees to insert in the payload
	insertableGuarantees, err := b.getInsertableGuarantees(parentID, finalID, finalized)
	if err != nil {
		return nil, err
	}

	// get the seals to insert in the payload
	insertableSeals, err := b.getInsertableSeals(parentID)
	if err != nil {
		return nil, err
	}

	// get the receipts to insert in the payload
	insertableReceipts, err := b.getInsertableReceipts(parentID, finalID, finalized)
	if err != nil {
		return nil, err
	}

	b.tracer.StartSpan(parentID, trace.CONBuildOnCreateHeader)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnCreateHeader)

	// build the payload so we can get the hash
	payload := &flow.Payload{
		Guarantees: insertableGuarantees,
		Seals:      insertableSeals,
		Receipts:   insertableReceipts,
	}

	parent, err := b.headers.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent: %w", err)
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
		Height:      parent.Height + 1,
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

// getInsertableGuarantees returns the list of CollectionGuarantees that should
// be inserted in the next payload. It looks in the collection mempool and
// applies the following filters:
//
// 1) If it was already included in the finalized part of the chain, remove it
//    from the memory pool and skip.
//
// 2) If it references an unknown block, remove it from the memory pool and
//    skip.
//
// 3) If the reference block has an expired height, also remove it from the
//    memory pool and skip.
//
// 4) If it was already included in the pending part of the chain, skip, but
//    keep in memory pool for now.
//
// 5) Otherwise, this guarantee can be included in the payload.
func (b *Builder) getInsertableGuarantees(parentID flow.Identifier,
	finalID flow.Identifier,
	finalHeight uint64) ([]*flow.CollectionGuarantee, error) {

	// STEP ONE: Create a lookup of all previously used guarantees on the part
	// of the chain that we are building on. We do this separately for pending
	// and finalized ancestors, so we can differentiate what to do about it.

	b.tracer.StartSpan(parentID, trace.CONBuildOnUnfinalizedLookup)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnUnfinalizedLookup)

	ancestorID := parentID
	pendingLookup := make(map[flow.Identifier]struct{})
	for ancestorID != finalID {
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		if ancestor.Height <= finalHeight {
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

	return guarantees, nil
}

// getInsertableSeals returns the list of Seals from the mempool that should be
// inserted in the next payload. It looks in the seal mempool and applies the
// following filters:
//
// 1) Do not collect more than maxSealCount items.
//
// 2) The seals should form a valid chain.
//
// 3) The seals should correspond to an incorporated result on this fork.
func (b *Builder) getInsertableSeals(parentID flow.Identifier) ([]*flow.Seal, error) {

	b.tracer.StartSpan(parentID, trace.CONBuildOnCreatePayloadSeals)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnCreatePayloadSeals)

	// get the parent's block seal, which constitutes the beginning of the
	// sealing chain; this is where we need to start with our chain of seals
	last, err := b.seals.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", parentID, err)
	}

	// get the last sealed block.
	sealed, err := b.headers.ByBlockID(last.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve sealed block (%x): %w", last.BlockID, err)
	}

	// the conensus matching engine can produce different seals for the same
	// ExecutionResult if it appeared in different blocks. Here we only want to
	// consider the seals that correspond to results and blocks on the current
	// fork.

	// filteredSeals is an index of block-height to seal, where we collect only
	// those seals from the mempool that correspond to IncorporatedResults on
	// this fork.
	filteredSeals := make(map[uint64]*flow.Seal)

	// loop through the fork, from parent to last sealed, inspect the payloads'
	// ExecutionResults, and check for matching IncorporatedResultSeals in the
	// mempool.
	ancestorID := parentID
	for ancestorID != sealed.ID() {

		ancestor, err := b.blocks.ByID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor (%x): %w", ancestorID, err)
		}

		// For each receipt in the block's payload, we recompose the
		// corresponding IncorporatedResult an check if we have a matching seal
		// in the mempool.
		for _, receipt := range ancestor.Payload.Receipts {

			// re-assemble the IncorporatedResult because we need its ID to
			// check if it is in the seal mempool.
			incorporatedResult := &flow.IncorporatedResult{
				// ATTENTION:
				// Here, IncorporatedBlockID should be set to ancestorID,
				// because that is the block in which the ExecutionResult is
				// contained. However, in phase 2 of the sealing roadmap, we are
				// still using a temporary sealing logic where the
				// IncorporatedBlockID is expected to be the result's block ID.
				IncorporatedBlockID: receipt.ExecutionResult.BlockID,
				Result:              &receipt.ExecutionResult,
			}

			// look for a seal that corresponds to this specific incorporated
			// result. This tells us that the seal is for a result on this fork,
			// and that it was calculated using the correct block ID for chunk
			// assignment (the IncorporatedBlockID).
			incorporatedResultSeal, ok := b.sealPool.ByID(incorporatedResult.ID())
			if ok {

				header, err := b.headers.ByBlockID(incorporatedResult.Result.BlockID)
				if err != nil {
					return nil, fmt.Errorf("could not get block for id (%x): %w", incorporatedResult.Result.BlockID, err)
				}

				filteredSeals[header.Height] = incorporatedResultSeal.Seal
			}
		}

		ancestorID = ancestor.Header.ParentID
	}

	// return immediadely if there are no seals to collect
	if len(filteredSeals) == 0 {
		return nil, nil
	}

	// now we need to collect only the seals that form a valid chain on top of
	// the last seal
	chain := make([]*flow.Seal, 0, len(filteredSeals))

	// start at last sealed height and stop when we have no seal for the next
	// block
	nextSealHeight := sealed.Height + 1
	nextSeal, ok := filteredSeals[nextSealHeight]
	for ok {
		chain = append(chain, nextSeal)
		nextSealHeight++
		nextSeal, ok = filteredSeals[nextSealHeight]
	}

	return chain, nil
}

// getInsertableReceipts returns the list of ExecutionReceipts that should be
// inserted in the next payload. It looks in the receipts mempool and applies
// the following filter:
//
// 1) If it corresponds to an unknown block, remove it from the mempool and
//    skip.
//
// 2) If the corresponding block was already sealed, remove it from the mempool
//    and skip.
//
// 3) If it was already included in the finalized part of the chain, remove it
//    from the memory pool and skip.
//
// 4) If it was already included in the pending part of the chain, skip, but
//    keep in memory pool for now.
//
// 5) If the receipt corresponds to a block that is not on this fork, skip, but
//    but keep in mempool for now.
//
// 6) Otherwise, this receipt can be included in the payload.
//
// Receipts have to be ordered by block height.
func (b *Builder) getInsertableReceipts(parentID flow.Identifier,
	finalID flow.Identifier,
	finalHeight uint64) ([]*flow.ExecutionReceipt, error) {

	var sealedHeight uint64
	err := b.db.View(operation.RetrieveSealedHeight(&sealedHeight))
	if err != nil {
		return nil, fmt.Errorf("could no retrieve sealed height: %w", err)
	}

	// forkBlocks is used to keep the IDs of the blocks we iterate through. We
	// use it to skip receipts that are not for blocks in the fork.
	forkBlocks := make(map[flow.Identifier]struct{})

	// Create a lookup table of all the receipts that are already inserted in
	// finalized unsealed blocks. This will be used to filter out duplicates.
	finalLookup := make(map[flow.Identifier]struct{})
	for height := sealedHeight + 1; height <= finalHeight; height++ {

		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get block for height (%d): %w", height, err)
		}

		index, err := b.index.ByBlockID(header.ID())
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", header.ID(), err)
		}

		forkBlocks[header.ID()] = struct{}{}

		for _, recID := range index.ReceiptIDs {
			finalLookup[recID] = struct{}{}
		}
	}

	// iterate through pending blocks, from parent to final, and keep track of
	// the receipts already recorded in those blocks.
	ancestorID := parentID
	pendingLookup := make(map[flow.Identifier]struct{})
	for ancestorID != finalID {
		forkBlocks[ancestorID] = struct{}{}

		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		if ancestor.Height <= finalHeight {
			return nil, fmt.Errorf("should always build on last finalized block")
		}
		index, err := b.index.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}
		for _, recID := range index.ReceiptIDs {
			pendingLookup[recID] = struct{}{}
		}
		ancestorID = ancestor.ParentID
	}

	// Go through mempool and collect valid receipts. We store them by block
	// height so as to sort them later. There can be multiple receipts per block
	// even if they correspond to the same result.
	receipts := make(map[uint64][]*flow.ExecutionReceipt) // [height] -> []receipt
	for _, receipt := range b.recPool.All() {

		// if block is unknown, remove from mempool and continue
		h, err := b.headers.ByBlockID(receipt.ExecutionResult.BlockID)
		if errors.Is(err, storage.ErrNotFound) {
			_ = b.recPool.Rem(receipt.ID())
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get reference block: %w", err)
		}

		// check whether receipt is for block at or below the sealed and
		// finalized height
		if h.Height <= sealedHeight {
			// Block has either already been sealed and finalized  _or_
			// block is a _sibling_ of a sealed and finalized block.
			// In either case, we don't need to seal the block anymore
			// and therefore can discard any ExecutionResults for it.
			_ = b.recPool.Rem(receipt.ID())
			continue
		}

		// if the receipt is already included in a finalized block, remove from
		// mempool and continue.
		_, ok := finalLookup[receipt.ID()]
		if ok {
			_ = b.recPool.Rem(receipt.ID())
			continue
		}

		// if the receipt is already included in a pending block, continue, but
		// don't remove from mempool.
		_, ok = pendingLookup[receipt.ID()]
		if ok {
			continue
		}

		// if the receipt is not for a block on this fork, continue, but don't
		// remove from mempool
		if _, ok := forkBlocks[receipt.ExecutionResult.BlockID]; !ok {
			continue
		}

		receipts[h.Height] = append(receipts[h.Height], receipt)
	}

	// sort receipts by block height
	sortedReceipts := sortReceipts(receipts)

	return sortedReceipts, nil
}

// sortReceipts takes a map of block-height to execution receipt, and returns
// the receipts in a slice sorted by block-height.
func sortReceipts(receipts map[uint64][]*flow.ExecutionReceipt) []*flow.ExecutionReceipt {

	keys := make([]uint64, 0, len(receipts))
	for k := range receipts {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	res := make([]*flow.ExecutionReceipt, 0, len(keys))
	for _, k := range keys {
		res = append(res, receipts[k]...)
	}

	return res
}

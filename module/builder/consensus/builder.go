// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow/filter/id"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// Builder is the builder for consensus block payloads. Upon providing a payload
// hash, it also memorizes which entities were included into the payload.
type Builder struct {
	metrics   module.MempoolMetrics
	tracer    module.Tracer
	db        *badger.DB
	state     protocol.MutableState
	seals     storage.Seals
	headers   storage.Headers
	index     storage.Index
	blocks    storage.Blocks
	resultsDB storage.ExecutionResults
	guarPool  mempool.Guarantees
	sealPool  mempool.IncorporatedResultSeals
	recPool   mempool.ExecutionTree
	cfg       Config
}

// NewBuilder creates a new block builder.
func NewBuilder(
	metrics module.MempoolMetrics,
	db *badger.DB,
	state protocol.MutableState,
	headers storage.Headers,
	seals storage.Seals,
	index storage.Index,
	blocks storage.Blocks,
	resultsDB storage.ExecutionResults,
	guarPool mempool.Guarantees,
	sealPool mempool.IncorporatedResultSeals,
	recPool mempool.ExecutionTree,
	tracer module.Tracer,
	options ...func(*Config),
) *Builder {

	// initialize default config
	cfg := Config{
		minInterval:       500 * time.Millisecond,
		maxInterval:       10 * time.Second,
		maxSealCount:      100,
		maxGuaranteeCount: 100,
		maxReceiptCount:   200,
		expiry:            flow.DefaultTransactionExpiry,
	}

	// apply option parameters
	for _, option := range options {
		option(&cfg)
	}

	b := &Builder{
		metrics:   metrics,
		db:        db,
		tracer:    tracer,
		state:     state,
		headers:   headers,
		seals:     seals,
		index:     index,
		blocks:    blocks,
		resultsDB: resultsDB,
		guarPool:  guarPool,
		sealPool:  sealPool,
		recPool:   recPool,
		cfg:       cfg,
	}
	return b
}

// BuildOn creates a new block header on top of the provided parent, using the
// given view and applying the custom setter function to allow the caller to
// make changes to the header before storing it.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {

	b.tracer.StartSpan(parentID, trace.CONBuildOn)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOn)

	// get the collection guarantees to insert in the payload
	insertableGuarantees, err := b.getInsertableGuarantees(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not insert guarantees: %w", err)
	}

	// get the receipts to insert in the payload
	insertableReceipts, err := b.getInsertableReceipts(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not insert receipts: %w", err)
	}

	// get the seals to insert in the payload
	insertableSeals, err := b.getInsertableSeals(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not insert seals: %w", err)
	}

	// assemble the block proposal
	proposal, err := b.createProposal(parentID,
		insertableGuarantees,
		insertableSeals,
		insertableReceipts,
		setter)
	if err != nil {
		return nil, fmt.Errorf("could not assemble proposal: %w", err)
	}

	b.tracer.StartSpan(parentID, trace.CONBuildOnDBInsert)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnDBInsert)

	err = b.state.Extend(proposal)
	if err != nil {
		return nil, fmt.Errorf("could not extend state with built proposal: %w", err)
	}

	return proposal.Header, nil
}

// getInsertableGuarantees returns the list of CollectionGuarantees that should
// be inserted in the next payload. It looks in the collection mempool and
// applies the following filters:
//
// 1) If it was already included in the fork, skip.
//
// 2) If it references an unknown block, skip.
//
// 3) If the referenced block has an expired height, skip.
//
// 4) Otherwise, this guarantee can be included in the payload.
func (b *Builder) getInsertableGuarantees(parentID flow.Identifier) ([]*flow.CollectionGuarantee, error) {
	b.tracer.StartSpan(parentID, trace.CONBuildOnCreatePayloadGuarantees)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnCreatePayloadGuarantees)

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

	// blockLookup keeps track of the blocks from limit to parent
	blockLookup := make(map[flow.Identifier]struct{})

	// receiptLookup keeps track of the receipts contained in blocks between
	// limit and parent
	receiptLookup := make(map[flow.Identifier]struct{})

	// loop through the fork backwards, from parent to limit, and keep track of
	// blocks and collections visited on the way
	ancestorID := parentID
	for {

		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}

		blockLookup[ancestorID] = struct{}{}

		index, err := b.index.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}

		for _, collID := range index.CollectionIDs {
			receiptLookup[collID] = struct{}{}
		}

		if ancestor.Height <= limit {
			break
		}

		ancestorID = ancestor.ParentID
	}

	// go through mempool and collect valid collections
	var guarantees []*flow.CollectionGuarantee
	for _, guarantee := range b.guarPool.All() {
		// add at most <maxGuaranteeCount> number of collection guarantees in a new block proposal
		// in order to prevent the block payload from being too big or computationally heavy for the
		// execution nodes
		if uint(len(guarantees)) >= b.cfg.maxGuaranteeCount {
			break
		}

		collID := guarantee.ID()

		// skip collections that are already included in a block on the fork
		_, duplicated := receiptLookup[collID]
		if duplicated {
			continue
		}

		// skip collections for blocks that are not within the limit
		_, ok := blockLookup[guarantee.ReferenceBlockID]
		if !ok {
			continue
		}

		guarantees = append(guarantees, guarantee)
	}

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

	// the consensus matching engine can produce different seals for the same
	// ExecutionResult if it appeared in different blocks. Here we only want to
	// consider the seals that correspond to results and blocks on the current
	// fork.

	// filteredSeals is an index of block-height to seal, where we collect only
	// those seals from the mempool that correspond to IncorporatedResults on
	// this fork.
	filteredSeals := make(map[uint64]*flow.IncorporatedResultSeal)

	// Walk backwards along the fork, from parent to last sealed, inspect the
	// payloads' ExecutionResults, and check for matching IncorporatedResultSeals
	// in the mempool.
	ancestorID := parentID
	sealedID := sealed.ID()
	for ancestorID != sealedID {

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
			// ATTENTION:
			// Here, IncorporatedBlockID (the first argument) should be set to
			// ancestorID, because that is the block that contains the
			// ExecutionResult. However, in phase 2 of the sealing roadmap, we
			// are still using a temporary sealing logic where the
			// IncorporatedBlockID is expected to be the result's block ID.
			incorporatedResult := flow.NewIncorporatedResult(
				receipt.ExecutionResult.BlockID,
				&receipt.ExecutionResult,
			)

			// look for a seal that corresponds to this specific incorporated
			// result. This tells us that the seal is for a result on this fork,
			// and that it was calculated using the correct block ID for chunk
			// assignment (the IncorporatedBlockID).
			irSeal, ok := b.sealPool.ByID(incorporatedResult.ID())
			if !ok {
				continue
			}

			header, err := b.headers.ByBlockID(incorporatedResult.Result.BlockID)
			if err != nil {
				return nil, fmt.Errorf("could not get block for id (%x): %w", incorporatedResult.Result.BlockID, err)
			}
			filteredSeals[header.Height] = irSeal
		}

		ancestorID = ancestor.Header.ParentID
	}

	// now we need to collect only the seals that form a valid chain on top of
	// the last seal
	chain := make([]*flow.Seal, 0, len(filteredSeals))

	// start at last sealed height and stop when we have no seal for the next
	// block
	nextSealHeight := sealed.Height + 1
	nextSeal, ok := filteredSeals[nextSealHeight]

	var count uint = 0
	for ok {
		// don't include more than maxSealCount seals
		if count >= b.cfg.maxSealCount {
			break
		}

		//  enforce that execution results form chain
		nextResultToBeSealed := nextSeal.IncorporatedResult.Result

		// at this point we are safe just to check this condition since every
		// ER gets validated by `module.ReceiptValidator` which checks if
		// results form a valid chain.
		if nextResultToBeSealed.PreviousResultID != last.ResultID {
			return nil, fmt.Errorf(
				"sealed execution results do not form chain, expect result ID %v, but got %v",
				last.ResultID,
				nextResultToBeSealed.PreviousResultID,
			)
		}

		last = nextSeal.Seal
		chain = append(chain, nextSeal.Seal)
		nextSealHeight++
		count++
		nextSeal, ok = filteredSeals[nextSealHeight]
	}

	return chain, nil
}

// getInsertableReceipts returns the list of ExecutionReceipts that should be
// inserted in the next payload. It looks in the receipts mempool and applies
// the following filter:
//
// 1) If it doesn't correspond to an unsealed block on the fork, skip it.
//
// 2) If it was already included in the fork, skip it.
//
// 3) Otherwise, this receipt can be included in the payload.
//
// Receipts have to be ordered by block height.
func (b *Builder) getInsertableReceipts(parentID flow.Identifier) ([]*flow.ExecutionReceipt, error) {
	b.tracer.StartSpan(parentID, trace.CONBuildOnCreatePayloadReceipts)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnCreatePayloadReceipts)

	// Get the latest sealed block on this fork, ie the highest block for which
	// there is a seal in this fork. This block is not necessarily finalized.
	latestSeal, err := b.seals.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", parentID, err)
	}
	sealedResult, err := b.resultsDB.ByID(latestSeal.ResultID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve sealed result (%x): %w", latestSeal.ResultID, err)
	}
	sealed, err := b.headers.ByBlockID(latestSeal.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve sealed block (%x): %w", latestSeal.BlockID, err)
	}

	// ancestors is used to keep the IDs of the ancestor blocks we iterate through.
	// We use it to skip receipts that are not for unsealed blocks in the fork.
	ancestors := make(map[flow.Identifier]struct{})

	// includedReceipts is a set of all receipts that are contained in unsealed blocks along the fork.
	includedReceipts := make(map[flow.Identifier]struct{})

	// loop through the fork backwards, from parent to last sealed (including),
	// and keep track of blocks and receipts visited on the way.
	sealedBlockID := sealed.ID()
	ancestorID := parentID
	for {
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		ancestors[ancestorID] = struct{}{}

		index, err := b.index.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}
		for _, recID := range index.ReceiptIDs {
			includedReceipts[recID] = struct{}{}
		}

		if ancestorID == sealedBlockID {
			break
		}
		ancestorID = ancestor.ParentID
	}

	// After recovering from a crash, the mempools are wiped and the sealed results will not
	// be stored in the Execution Tree anymore. Adding the result to the tree allows to create
	// a vertex in the tree without attaching any Execution Receipts to it. Thereby, we can
	// traverse to receipts committing to derived results without having to find the receipts
	// for the sealed result.
	err = b.recPool.AddResult(sealedResult, sealed) // no-op, if result is already in Execution Tree
	if err != nil {
		return nil, fmt.Errorf("failed to add sealed result as vertex to ExecutionTree (%x): %w", latestSeal.ResultID, err)
	}
	isResultForUnsealedBlock := isResultForBlock(ancestors)
	isReceiptUniqueAndUnsealed := isNoDupAndNotSealed(includedReceipts, sealedBlockID)
	// find all receipts:
	// 1) whose result connects all the way to the last sealed result
	// 2) is unique (never seen in unsealed blocks)
	receipts, err := b.recPool.ReachableReceipts(latestSeal.ResultID, isResultForUnsealedBlock, isReceiptUniqueAndUnsealed)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve reachable receipts from memool: %w", err)
	}

	// don't collect more than maxReceiptCount receipts
	if uint(len(receipts)) > b.cfg.maxReceiptCount {
		receipts = receipts[:b.cfg.maxReceiptCount]
	}

	return receipts, nil
}

// createProposal assembles a block with the provided header and payload
// information
func (b *Builder) createProposal(parentID flow.Identifier,
	guarantees []*flow.CollectionGuarantee,
	seals []*flow.Seal,
	receipts []*flow.ExecutionReceipt,
	setter func(*flow.Header) error) (*flow.Block, error) {

	b.tracer.StartSpan(parentID, trace.CONBuildOnCreateHeader)
	defer b.tracer.FinishSpan(parentID, trace.CONBuildOnCreateHeader)

	// build the payload so we can get the hash
	payload := &flow.Payload{
		Guarantees: guarantees,
		Seals:      seals,
		Receipts:   receipts,
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

	proposal := &flow.Block{
		Header:  header,
		Payload: payload,
	}

	return proposal, nil
}

// isResultForBlock constructs a mempool.BlockFilter that accepts only blocks whose ID is part of the given set.
func isResultForBlock(blockIDs map[flow.Identifier]struct{}) mempool.BlockFilter {
	blockIdFilter := id.InSet(blockIDs)
	return func(h *flow.Header) bool {
		return blockIdFilter(h.ID())
	}
}

// isNoDupAndNotSealed constructs a mempool.ReceiptFilter for discarding receipts that
// * are duplicates
// * or are for the sealed block
func isNoDupAndNotSealed(includedReceipts map[flow.Identifier]struct{}, sealedBlockID flow.Identifier) mempool.ReceiptFilter {
	return func(receipt *flow.ExecutionReceipt) bool {
		if _, duplicate := includedReceipts[receipt.ID()]; duplicate {
			return false
		}
		if receipt.ExecutionResult.BlockID == sealedBlockID {
			return false
		}
		return true
	}
}

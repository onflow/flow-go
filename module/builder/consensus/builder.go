// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Builder is the builder for consensus block payloads. Upon providing a payload
// hash, it also memorizes which entities were included into the payload.
type Builder struct {
	metrics  module.MempoolMetrics
	db       *badger.DB
	seals    storage.Seals
	headers  storage.Headers
	index    storage.Index
	blocks   storage.Blocks
	guarPool mempool.Guarantees
	sealPool mempool.Seals
	recPool  mempool.Receipts
	cfg      Config
}

// NewBuilder creates a new block builder.
func NewBuilder(metrics module.MempoolMetrics,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	index storage.Index,
	blocks storage.Blocks,
	guarPool mempool.Guarantees,
	sealPool mempool.Seals,
	recPool mempool.Receipts,
	options ...func(*Config)) *Builder {

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

// BuildOn creates a new block header built on the provided parent, using the
// given view and applying the custom setter function to allow the caller to
// make changes to the header before storing it.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {

	parent, err := b.headers.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent: %w", err)
	}

	// nextHeight is the height of the new header we are building
	nextHeight := parent.Height + 1

	// create the payload with guarantees, seals, and receipts, taken from the
	// mempools.
	payload, lastSeal, err := b.buildNextPayload(parentID, nextHeight)
	if err != nil {
		return nil, fmt.Errorf("could not build payload")
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
		Height:      nextHeight,
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
	err = b.blocks.Store(proposal)
	if err != nil {
		return nil, fmt.Errorf("could not store proposal: %w", err)
	}

	// update protocol state index for the seal and initialize children index
	blockID := proposal.ID()
	err = operation.RetryOnConflict(b.db.Update, func(tx *badger.Txn) error {

		// Index the last sealed height for this block, so that we could know
		// the highest sealed block at this block. The last sealed height is
		// useful for matching engine to reject execution results or result
		// approvals if the block has already been sealed.
		err = operation.IndexBlockSeal(blockID, lastSeal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index proposal seal: %w", err)
		}

		// index the child block for recovery
		err = procedure.IndexNewBlock(blockID, proposal.Header.ParentID)(tx)
		if err != nil {
			return fmt.Errorf("could not index new block: %w", err)
		}
		return nil
	})

	return header, err
}

// buildNextPayload creates the payload of the next block with guarantees,
// seals, and receipts taken from the mempools. It returns the payload and the
// last created seal.
//
// TODO :
//
// * Used items should not be removed from the mempools until we are sure that
//   they were correctly inserted in the payload. We should make this an atomic
//   operation.
//
// * Use a common loop for finding duplicates in the blockchain for guarantees,
//   seals, and receipts.
func (b *Builder) buildNextPayload(parentID flow.Identifier, nextHeight uint64) (*flow.Payload, *flow.Seal, error) {

	var finalHeight uint64
	err := b.db.View(operation.RetrieveFinalizedHeight(&finalHeight))
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve finalized height: %w", err)
	}

	var finalID flow.Identifier
	err = b.db.View(operation.LookupBlockHeight(finalHeight, &finalID))
	if err != nil {
		return nil, nil, fmt.Errorf("could not lookup finalized block: %w", err)
	}

	// get list of guarantees to insert
	guarantees, err := b.getInsertableGuarantees(parentID, finalID, finalHeight, nextHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve the list of guarantees to insert in next payload")
	}

	// get list of seals to insert
	seals, lastSeal, err := b.getInsertableSeals(parentID, finalID, finalHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve the list of seals to insert in next payload")
	}

	// get list of receipts to insert
	receipts, err := b.getInsertableReceipts(parentID, finalID, finalHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve the list of receipts to insert in nex payload")
	}

	// build the payload so we can get the hash
	payload := &flow.Payload{
		Identities: nil,
		Guarantees: guarantees,
		Seals:      seals,
		Receipts:   receipts,
	}

	return payload, lastSeal, nil
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
func (b *Builder) getInsertableGuarantees(
	parentID flow.Identifier,
	finalID flow.Identifier,
	finalHeight uint64,
	nextHeight uint64) ([]*flow.CollectionGuarantee, error) {

	// Create a lookup of all previously used guarantees on the part of the
	// chain that we are building on. We do this separately for pending and
	// finalized ancestors, so we can differentiate what to do about it.

	// iterate through pending blocks, from parent to final, and keep track of
	// the collections aready recorded in those blocks.
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

	// When it comes to finalized blocks, we look back only as far as the expiry
	// limit; any guarantee with a reference block before that can not be
	// included anymore anyway
	limit := nextHeight - uint64(b.cfg.expiry)
	if limit > nextHeight { // overflow check
		limit = 0
	}

	// Look up the root height so we don't look too far back. Initially this is
	// the genesis block height (aka 0).
	var rootHeight uint64
	err := b.db.View(operation.RetrieveRootHeight(&rootHeight))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root block height: %w", err)
	}
	if limit < rootHeight {
		limit = rootHeight
	}

	// Iterate through finalized blocks, from final to limit, and keep track of
	// the collections already recorded in those blocks.
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

	// Apply filter to collection mempool.
	var guarantees []*flow.CollectionGuarantee
	for _, guarantee := range b.guarPool.All() {
		collID := guarantee.ID()

		// already included in finalized part of the chain
		_, duplicated := finalLookup[collID]
		if duplicated {
			_ = b.guarPool.Rem(collID)
			continue
		}

		// unknown block
		ref, err := b.headers.ByBlockID(guarantee.ReferenceBlockID)
		if errors.Is(err, storage.ErrNotFound) {
			_ = b.guarPool.Rem(collID)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get reference block: %w", err)
		}

		// expired height
		if ref.Height < limit {
			_ = b.guarPool.Rem(collID)
			continue
		}

		// already included in pending part of the chain
		_, duplicated = pendingLookup[collID]
		if duplicated {
			continue
		}

		guarantees = append(guarantees, guarantee)
	}

	b.metrics.MempoolEntries(metrics.ResourceGuarantee, b.guarPool.Size())

	return guarantees, nil
}

// getInsertableSeals returns the list of Seals from the mempool that should be
// inserted in the next payload, as well as the last created seal (which doesn't
// necessarily belong to the insertable seals if nothing was taken from the
// mempool).  It looks in the seal mempool and applies the following filters:
//
// 1) Do not collect more than maxSealCount items.
//
// 2) The seals should form a valid chain.
func (b *Builder) getInsertableSeals(
	parentID flow.Identifier,
	finalID flow.Identifier,
	finalHeight uint64) ([]*flow.Seal, *flow.Seal, error) {

	// STEP ONE: We try to get all ancestors from last sealed block all the way
	// to the parent. Then we try to get seals for each of them until we don't
	// find one. This creates a valid chain of seals from the last sealed block
	// to at most the parent.

	// create a mapping of block to seal for all seals in our pool
	byBlock := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range b.sealPool.All() {
		byBlock[seal.BlockID] = seal
	}
	if int(b.sealPool.Size()) > len(byBlock) {
		return nil, nil, fmt.Errorf("multiple seals for the same block")
	}

	var sealedHeight uint64
	err := b.db.View(operation.RetrieveSealedHeight(&sealedHeight))
	if err != nil {
		return nil, nil, fmt.Errorf("could no retrieve sealed height: %w", err)
	}

	var sealedID flow.Identifier
	err = b.db.View(operation.LookupBlockHeight(sealedHeight, &sealedID))
	if err != nil {
		return nil, nil, fmt.Errorf("could not lookup sealed block: %w", err)
	}

	lastSeal, err := b.seals.ByBlockID(sealedID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve last seal (%x): %w", sealedID, err)
	}

	// we now go from last sealed height plus one to finalized height and check
	// if we have the seal for each of them step by step; often we will not even
	// enter this loop, because last sealed height is higher than finalized.
	stop := false
	var seals []*flow.Seal
	var sealCount uint
	for height := sealedHeight + 1; height <= finalHeight; height++ {
		if len(byBlock) == 0 {
			break
		}

		// add at most <maxSealCount> number of seals in a new block proposal in
		// order to prevent the block payload from being too big.
		if sealCount >= b.cfg.maxSealCount {
			stop = true
			break
		}

		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get block for height (%d): %w", height, err)
		}

		blockID := header.ID()

		next, found := byBlock[blockID]
		if !found {
			stop = true
			break
		}

		if !bytes.Equal(next.InitialState, lastSeal.FinalState) {
			return nil, nil, fmt.Errorf("seal execution states do not connect in finalized")
		}

		seals = append(seals, next)

		sealCount++

		delete(byBlock, blockID)

		lastSeal = next
	}

	// NOTE: We should only run the next part in case we did not use up all
	// seals in the previous part; both break cases should make us skip the rest
	// as it means we either ran out of seals or we can't find the next link in
	// the chain.
	if !stop {
		// Once we have filled in seals for all finalized blocks we need to
		// check the non-finalized blocks backwards; collect all of them, from
		// direct parent to just before finalized, and see if we can use up the
		// rest of the seals. We need to be careful to break when reaching the
		// last sealed block as it could be higher than the last finalized
		// block.
		ancestorID := parentID
		var pendingIDs []flow.Identifier
		for ancestorID != finalID && ancestorID != lastSeal.BlockID {
			pendingIDs = append(pendingIDs, ancestorID)
			ancestor, err := b.headers.ByBlockID(ancestorID)
			if err != nil {
				return nil, nil, fmt.Errorf("could not get sealable ancestor (%x): %w", ancestorID, err)
			}
			ancestorID = ancestor.ParentID
		}

		for i := len(pendingIDs) - 1; i >= 0; i-- {
			if len(byBlock) == 0 {
				break
			}

			// add at most <maxSealCount> number of seals in a new block
			// proposal in order to prevent the block payload from being too
			// big.
			if sealCount >= b.cfg.maxSealCount {
				break
			}

			pendingID := pendingIDs[i]

			next, found := byBlock[pendingID]
			if !found {
				break
			}

			if !bytes.Equal(next.InitialState, lastSeal.FinalState) {
				return nil, nil, fmt.Errorf("seal execution states do not connect in pending")
			}

			seals = append(seals, next)

			sealCount++

			delete(byBlock, pendingID)

			lastSeal = next
		}
	}

	return seals, lastSeal, nil
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
// 5) Otherwise, this receipt can be included in the payload.
//
// 6) Receipts have to be ordered by block height.
func (b *Builder) getInsertableReceipts(parentID flow.Identifier,
	finalID flow.Identifier,
	finalHeight uint64) ([]*flow.ExecutionReceipt, error) {

	var sealedHeight uint64
	err := b.db.View(operation.RetrieveSealedHeight(&sealedHeight))
	if err != nil {
		return nil, fmt.Errorf("could no retrieve sealed height: %w", err)
	}

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

		for _, recID := range index.ReceiptIDs {
			finalLookup[recID] = struct{}{}
		}
	}

	// iterate through pending blocks, from parent to final, and keep track of
	// the receipts aready recorded in those blocks.
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
		for _, recID := range index.ReceiptIDs {
			pendingLookup[recID] = struct{}{}
		}
		ancestorID = ancestor.ParentID
	}

	// Go through mempool and collect valid receipts. We store them by block
	// height so as to sort them later.
	receipts := make(map[uint64]*flow.ExecutionReceipt) // [height] -> receipt
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

		// height is used for sorting
		height := h.Height

		// if block is already sealed, remove from mempool and continue
		_, err = b.seals.ByBlockID(receipt.ExecutionResult.BlockID)
		if err == nil {
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

		_ = b.recPool.Rem(receipt.ID())
		receipts[height] = receipt
	}

	// sort receipts by block height
	sortedReceipts := sortReceipts(receipts)

	return sortedReceipts, nil
}

// sortReceipts takes a map of block-height to execution receipt, and returns
// the receipts in a slice sorted by block-height.
func sortReceipts(receipts map[uint64]*flow.ExecutionReceipt) []*flow.ExecutionReceipt {

	keys := make([]uint64, 0, len(receipts))
	for k := range receipts {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	res := make([]*flow.ExecutionReceipt, 0, len(keys))
	for _, k := range keys {
		res = append(res, receipts[k])
	}

	return res
}

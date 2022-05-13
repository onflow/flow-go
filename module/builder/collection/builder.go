package collection

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/opentracing/opentracing-go"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

// Builder is the builder for collection block payloads. Upon providing a
// payload hash, it also memorizes the payload contents.
//
// NOTE: Builder is NOT safe for use with multiple goroutines. Since the
// HotStuff event loop is the only consumer of this interface and is single
// threaded, this is OK.
type Builder struct {
	db             *badger.DB
	mainHeaders    storage.Headers
	clusterHeaders storage.Headers
	payloads       storage.ClusterPayloads
	transactions   mempool.Transactions
	tracer         module.Tracer
	config         Config
}

func NewBuilder(db *badger.DB, tracer module.Tracer, mainHeaders storage.Headers, clusterHeaders storage.Headers, payloads storage.ClusterPayloads, transactions mempool.Transactions, opts ...Opt) (*Builder, error) {

	b := Builder{
		db:             db,
		tracer:         tracer,
		mainHeaders:    mainHeaders,
		clusterHeaders: clusterHeaders,
		payloads:       payloads,
		transactions:   transactions,
		config:         DefaultConfig(),
	}

	for _, apply := range opts {
		apply(&b.config)
	}

	// sanity check config
	if b.config.ExpiryBuffer >= flow.DefaultTransactionExpiry {
		return nil, fmt.Errorf("invalid configured expiry buffer exceeds tx expiry (%d > %d)", b.config.ExpiryBuffer, flow.DefaultTransactionExpiry)
	}

	return &b, nil
}

// BuildOn creates a new block built on the given parent. It produces a payload
// that is valid with respect to the un-finalized chain it extends.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {
	var proposal cluster.Block                 // proposal we are building
	var parent flow.Header                     // parent of the proposal we are building
	var clusterChainFinalizedBlock flow.Header // finalized block on the cluster chain
	var refChainFinalizedHeight uint64         // finalized height on reference chain
	var refChainFinalizedID flow.Identifier    // finalized block ID on reference chain

	startTime := time.Now()

	// STEP ONE: build a lookup for excluding duplicated transactions.
	// This is briefly how it works:
	//
	// Let E be the global transaction expiry.
	// When incorporating a new collection C, with reference height R, we enforce
	// that it contains only transactions with reference heights in [R,R+E).
	// * if we are building C:
	//   * we don't build expired collections (ie. our local finalized consensus height is at most R+E-1)
	//   * we don't include transactions referencing un-finalized blocks
	//   * therefore, C will contain only transactions with reference heights in [R,R+E)
	// * if we are validating C:
	//   * honest validators only consider C valid if all its transactions have reference heights in [R,R+E)
	//
	// Therefore, to check for duplicates, we only need a lookup for transactions in collection
	// with expiry windows that overlap with our collection under construction.
	//
	// A collection with overlapping expiry window can be finalized or un-finalized.
	// * to find all non-expired and finalized collections, we make use of an index
	//   (main_chain_finalized_height -> cluster_block_id), to search for a range of main chain heights
	//   which could be only referenced by collections with overlapping expiry windows.
	// * to find all overlapping and un-finalized collections, we can't use the above index, because it's
	//   only for finalized collections. Instead, we simply traverse along the chain up to the last
	//   finalized block. This could possibly include some collections with expiry windows that DON'T
	//   overlap with our collection under construction, but it is unlikely and doesn't impact correctness.
	//
	// After combining both the finalized and un-finalized cluster blocks that overlap with our expiry window,
	// we can iterate through their transactions, and build a lookup for excluding duplicated transactions.
	err := b.db.View(func(btx *badger.Txn) error {

		// TODO (ramtin): enable this again
		// b.tracer.StartSpan(parentID, trace.COLBuildOnSetup)
		// defer b.tracer.FinishSpan(parentID, trace.COLBuildOnSetup)

		err := operation.RetrieveHeader(parentID, &parent)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent: %w", err)
		}

		// retrieve the height and ID of the latest finalized block ON THE MAIN CHAIN
		// this is used as the reference point for transaction expiry
		err = operation.RetrieveFinalizedHeight(&refChainFinalizedHeight)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve main finalized height: %w", err)
		}
		err = operation.LookupBlockHeight(refChainFinalizedHeight, &refChainFinalizedID)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve main finalized ID: %w", err)
		}

		// retrieve the finalized boundary ON THE CLUSTER CHAIN
		err = procedure.RetrieveLatestFinalizedClusterHeader(parent.ChainID, &clusterChainFinalizedBlock)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve cluster final: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// pre-compute the minimum possible reference block height for transactions
	// included in this collection (actual reference height may be greater)
	minPossibleRefHeight := refChainFinalizedHeight - uint64(flow.DefaultTransactionExpiry-b.config.ExpiryBuffer)
	if minPossibleRefHeight > refChainFinalizedHeight {
		minPossibleRefHeight = 0 // overflow check
	}

	// TODO (ramtin): enable this again
	// b.tracer.FinishSpan(parentID, trace.COLBuildOnSetup)
	// b.tracer.StartSpan(parentID, trace.COLBuildOnUnfinalizedLookup)
	// defer b.tracer.FinishSpan(parentID, trace.COLBuildOnUnfinalizedLookup)

	// STEP TWO: create a lookup of all previously used transactions on the
	// part of the chain we care about. We do this separately for
	// un-finalized and finalized sections of the chain to decide whether to
	// remove conflicting transactions from the mempool.

	// keep track of transactions in the ancestry to avoid duplicates
	lookup := newTransactionLookup()
	// keep track of transactions to enforce rate limiting
	limiter := newRateLimiter(b.config, parent.Height+1)

	// RATE LIMITING: the builder module can be configured to limit the
	// rate at which transactions with a common payer are included in
	// blocks. Depending on the configured limit, we either allow 1
	// transaction every N sequential collections, or we allow K transactions
	// per collection.

	// first, look up previously included transactions in UN-FINALIZED ancestors
	err = b.populateUnfinalizedAncestryLookup(parentID, clusterChainFinalizedBlock.Height, lookup, limiter)
	if err != nil {
		return nil, fmt.Errorf("could not populate un-finalized ancestry lookout (parent_id=%x): %w", parentID, err)
	}

	// TODO (ramtin): enable this again
	// b.tracer.FinishSpan(parentID, trace.COLBuildOnUnfinalizedLookup)
	// b.tracer.StartSpan(parentID, trace.COLBuildOnFinalizedLookup)
	// defer b.tracer.FinishSpan(parentID, trace.COLBuildOnFinalizedLookup)

	// second, look up previously included transactions in FINALIZED ancestors
	err = b.populateFinalizedAncestryLookup(minPossibleRefHeight, refChainFinalizedHeight, lookup, limiter)
	if err != nil {
		return nil, fmt.Errorf("could not populate finalized ancestry lookup: %w", err)
	}

	// TODO (ramtin): enable this again
	// b.tracer.FinishSpan(parentID, trace.COLBuildOnFinalizedLookup)
	// b.tracer.StartSpan(parentID, trace.COLBuildOnCreatePayload)
	// defer b.tracer.FinishSpan(parentID, trace.COLBuildOnCreatePayload)

	// STEP THREE: build a payload of valid transactions, while at the same
	// time figuring out the correct reference block ID for the collection.

	// keep track of the actual smallest reference height of all included transactions
	minRefHeight := uint64(math.MaxUint64)
	minRefID := refChainFinalizedID

	var transactions []*flow.TransactionBody
	var totalByteSize uint64
	var totalGas uint64
	for _, tx := range b.transactions.All() {

		// if we have reached maximum number of transactions, stop
		if uint(len(transactions)) >= b.config.MaxCollectionSize {
			break
		}

		txByteSize := uint64(tx.ByteSize())
		// ignore transactions with tx byte size bigger that the max amount per collection
		// this case shouldn't happen ever since we keep a limit on tx byte size but in case
		// we keep this condition
		if txByteSize > b.config.MaxCollectionByteSize {
			continue
		}

		// because the max byte size per tx is way smaller than the max collection byte size, we can stop here and not continue.
		// to make it more effective in the future we can continue adding smaller ones
		if totalByteSize+txByteSize > b.config.MaxCollectionByteSize {
			break
		}

		// ignore transactions with max gas bigger that the max total gas per collection
		// this case shouldn't happen ever but in case we keep this condition
		if tx.GasLimit > b.config.MaxCollectionTotalGas {
			continue
		}

		// cause the max gas limit per tx is way smaller than the total max gas per collection, we can stop here and not continue.
		// to make it more effective in the future we can continue adding smaller ones
		if totalGas+tx.GasLimit > b.config.MaxCollectionTotalGas {
			break
		}

		// retrieve the main chain header that was used as reference
		refHeader, err := b.mainHeaders.ByBlockID(tx.ReferenceBlockID)
		if errors.Is(err, storage.ErrNotFound) {
			continue // in case we are configured with liberal transaction ingest rules
		}
		if err != nil {
			return nil, fmt.Errorf("could not retrieve reference header: %w", err)
		}

		// disallow un-finalized reference blocks
		if refChainFinalizedHeight < refHeader.Height {
			continue
		}
		// make sure the reference block is finalized and not orphaned
		blockIDFinalizedAtReferenceHeight, err := b.mainHeaders.BlockIDByHeight(refHeader.Height)
		if err != nil {
			return nil, fmt.Errorf("could not check that reference block (id=%x) is finalized: %w", tx.ReferenceBlockID, err)
		}
		if blockIDFinalizedAtReferenceHeight != tx.ReferenceBlockID {
			// the transaction references an orphaned block - it will never be valid
			b.transactions.Rem(tx.ID())
			continue
		}

		// ensure the reference block is not too old
		if refHeader.Height < minPossibleRefHeight {
			// the transaction is expired, it will never be valid
			b.transactions.Rem(tx.ID())
			continue
		}

		txID := tx.ID()
		// check that the transaction was not already used in un-finalized history
		if lookup.isUnfinalizedAncestor(txID) {
			continue
		}

		// check that the transaction was not already included in finalized history.
		if lookup.isFinalizedAncestor(txID) {
			// remove from mempool, conflicts with finalized block will never be valid
			b.transactions.Rem(txID)
			continue
		}

		// enforce rate limiting rules
		if limiter.shouldRateLimit(tx) {
			continue
		}

		// ensure we find the lowest reference block height
		if refHeader.Height < minRefHeight {
			minRefHeight = refHeader.Height
			minRefID = tx.ReferenceBlockID
		}

		// update per-payer transaction count
		limiter.transactionIncluded(tx)

		transactions = append(transactions, tx)
		totalByteSize += txByteSize
		totalGas += tx.GasLimit
	}

	// STEP FOUR: we have a set of transactions that are valid to include
	// on this fork. Now we need to create the collection that will be
	// used in the payload and construct the final proposal model
	// TODO (ramtin): enable this again
	// b.tracer.FinishSpan(parentID, trace.COLBuildOnCreatePayload)
	// b.tracer.StartSpan(parentID, trace.COLBuildOnCreateHeader)
	// defer b.tracer.FinishSpan(parentID, trace.COLBuildOnCreateHeader)

	// build the payload from the transactions
	payload := cluster.PayloadFromTransactions(minRefID, transactions...)

	header := flow.Header{
		ChainID:     parent.ChainID,
		ParentID:    parentID,
		Height:      parent.Height + 1,
		PayloadHash: payload.Hash(),
		Timestamp:   time.Now().UTC(),

		// NOTE: we rely on the HotStuff-provided setter to set the other
		// fields, which are related to signatures and HotStuff internals
	}

	// set fields specific to the consensus algorithm
	err = setter(&header)
	if err != nil {
		return nil, fmt.Errorf("could not set fields to header: %w", err)
	}

	proposal = cluster.Block{
		Header:  &header,
		Payload: &payload,
	}

	// TODO (ramtin): enable this again
	// b.tracer.FinishSpan(parentID, trace.COLBuildOnCreateHeader)

	span, ctx, _ := b.tracer.StartCollectionSpan(context.Background(), proposal.ID(), trace.COLBuildOn, opentracing.StartTime(startTime))
	defer span.Finish()

	dbInsertSpan, _ := b.tracer.StartSpanFromContext(ctx, trace.COLBuildOnDBInsert)
	defer dbInsertSpan.Finish()

	// finally we insert the block in a write transaction
	err = operation.RetryOnConflict(b.db.Update, procedure.InsertClusterBlock(&proposal))
	if err != nil {
		return nil, fmt.Errorf("could not insert built block: %w", err)
	}

	return proposal.Header, nil
}

// populateUnfinalizedAncestryLookup traverses the unfinalized ancestry backward
// to populate the transaction lookup (used for deduplication) and the rate limiter
// (used to limit transaction submission by payer).
//
// The traversal begins with the block specified by parentID (the block we are
// building on top of) and ends with the oldest unfinalized block in the ancestry.
func (b *Builder) populateUnfinalizedAncestryLookup(parentID flow.Identifier, finalHeight uint64, lookup *transactionLookup, limiter *rateLimiter) error {

	err := fork.TraverseBackward(b.clusterHeaders, parentID, func(ancestor *flow.Header) error {
		payload, err := b.payloads.ByBlockID(ancestor.ID())
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor payload: %w", err)
		}

		for _, tx := range payload.Collection.Transactions {
			lookup.addUnfinalizedAncestor(tx.ID())
			limiter.addAncestor(ancestor.Height, tx)
		}
		return nil
	}, fork.ExcludingHeight(finalHeight))

	return err
}

// populateFinalizedAncestryLookup traverses the reference block height index to
// populate the transaction lookup (used for deduplication) and the rate limiter
// (used to limit transaction submission by payer).
//
// The traversal is structured so that we check every collection whose reference
// block height translates to a possible constituent transaction which could also
// appear in the collection we are building.
func (b *Builder) populateFinalizedAncestryLookup(minRefHeight, maxRefHeight uint64, lookup *transactionLookup, limiter *rateLimiter) error {

	// Let E be the global transaction expiry constant, measured in blocks. For each
	// T ∈ `includedTransactions`, we have to decide whether the transaction
	// already appeared in _any_ finalized cluster block.
	// Notation:
	//   - consider a valid cluster block C and let c be its reference block height
	//   - consider a transaction T ∈ `includedTransactions` and let t denote its
	//     reference block height
	//
	// Boundary conditions:
	// 1. C's reference block height is equal to the lowest reference block height of
	//    all its constituent transactions. Hence, C can contain T if and only if c <= t.
	// 2. a transaction with reference block height T is eligible for inclusion in
	//    any collection with reference block height C where C<=T<C+E
	//
	// Therefore, to guarantee that we find all possible duplicates, we need to check
	// all collections with reference block height in the range (C-E,C+E). Since we
	// know the actual maximum reference height included in this collection, we can
	// further limit the search range to (C-E,maxRefHeight]

	// the finalized cluster blocks which could possibly contain any conflicting transactions
	var clusterBlockIDs []flow.Identifier
	start, end := findRefHeightSearchRangeForConflictingClusterBlocks(minRefHeight, maxRefHeight)
	err := b.db.View(operation.LookupClusterBlocksByReferenceHeightRange(start, end, &clusterBlockIDs))
	if err != nil {
		return fmt.Errorf("could not lookup finalized cluster blocks by reference height range [%d,%d]: %w", start, end, err)
	}

	for _, blockID := range clusterBlockIDs {
		header, err := b.clusterHeaders.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve cluster header (id=%x): %w", blockID, err)
		}
		payload, err := b.payloads.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve cluster payload (block_id=%x): %w", blockID, err)
		}
		for _, tx := range payload.Collection.Transactions {
			lookup.addFinalizedAncestor(tx.ID())
			limiter.addAncestor(header.Height, tx)
		}
	}

	return nil
}

// findRefHeightSearchRangeForConflictingClusterBlocks computes the range of reference
// block heights of ancestor blocks which could possibly contain transactions
// duplicating those in our collection under construction, based on the range of
// reference heights of transactions in the collection under construction.
//
// Input range is the (inclusive) range of reference heights of transactions included
// in the collection under construction. Output range is the (inclusive) range of
// reference heights which need to be searched.
func findRefHeightSearchRangeForConflictingClusterBlocks(minRefHeight, maxRefHeight uint64) (start, end uint64) {
	start = minRefHeight - flow.DefaultTransactionExpiry + 1
	if start > minRefHeight {
		start = 0 // overflow check
	}
	end = maxRefHeight
	return start, end
}

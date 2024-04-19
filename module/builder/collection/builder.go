package collection

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/trace"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/utils/logging"
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
	protoState     protocol.State
	clusterState   clusterstate.State
	payloads       storage.ClusterPayloads
	transactions   mempool.Transactions
	tracer         module.Tracer
	config         Config
	log            zerolog.Logger
	clusterEpoch   uint64 // the operating epoch for this cluster
	// cache of values about the operating epoch which never change
	refEpochFirstHeight uint64           // first height of this cluster's operating epoch
	epochFinalHeight    *uint64          // last height of this cluster's operating epoch (nil if epoch not ended)
	epochFinalID        *flow.Identifier // ID of last block in this cluster's operating epoch (nil if epoch not ended)
}

func NewBuilder(
	db *badger.DB,
	tracer module.Tracer,
	protoState protocol.State,
	clusterState clusterstate.State,
	mainHeaders storage.Headers,
	clusterHeaders storage.Headers,
	payloads storage.ClusterPayloads,
	transactions mempool.Transactions,
	log zerolog.Logger,
	epochCounter uint64,
	opts ...Opt,
) (*Builder, error) {
	b := Builder{
		db:             db,
		tracer:         tracer,
		protoState:     protoState,
		clusterState:   clusterState,
		mainHeaders:    mainHeaders,
		clusterHeaders: clusterHeaders,
		payloads:       payloads,
		transactions:   transactions,
		config:         DefaultConfig(),
		log:            log.With().Str("component", "cluster_builder").Logger(),
		clusterEpoch:   epochCounter,
	}

	err := db.View(operation.RetrieveEpochFirstHeight(epochCounter, &b.refEpochFirstHeight))
	if err != nil {
		return nil, fmt.Errorf("could not get epoch first height: %w", err)
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
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error, sign func(*flow.Header) error) (*flow.Header, error) {
	parentSpan, ctx := b.tracer.StartSpanFromContext(context.Background(), trace.COLBuildOn)
	defer parentSpan.End()

	// STEP 1: build a lookup for excluding duplicated transactions.
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
	//   (main_chain_finalized_height -> cluster_block_ids with respective reference height), to search for a range of main chain heights
	//   which could be only referenced by collections with overlapping expiry windows.
	// * to find all overlapping and un-finalized collections, we can't use the above index, because it's
	//   only for finalized collections. Instead, we simply traverse along the chain up to the last
	//   finalized block. This could possibly include some collections with expiry windows that DON'T
	//   overlap with our collection under construction, but it is unlikely and doesn't impact correctness.
	//
	// After combining both the finalized and un-finalized cluster blocks that overlap with our expiry window,
	// we can iterate through their transactions, and build a lookup for excluding duplicated transactions.
	//
	// RATE LIMITING: the builder module can be configured to limit the
	// rate at which transactions with a common payer are included in
	// blocks. Depending on the configured limit, we either allow 1
	// transaction every N sequential collections, or we allow K transactions
	// per collection. The rate limiter tracks transactions included previously
	// to enforce rate limit rules for the constructed block.

	span, _ := b.tracer.StartSpanFromContext(ctx, trace.COLBuildOnGetBuildCtx)
	buildCtx, err := b.getBlockBuildContext(parentID)
	span.End()
	if err != nil {
		return nil, fmt.Errorf("could not get block build context: %w", err)
	}

	log := b.log.With().
		Hex("parent_id", parentID[:]).
		Str("chain_id", buildCtx.parent.ChainID.String()).
		Uint64("final_ref_height", buildCtx.refChainFinalizedHeight).
		Logger()
	log.Debug().Msg("building new cluster block")

	// STEP 1a: create a lookup of all transactions included in UN-FINALIZED ancestors.
	// In contrast to the transactions collected in step 1b, transactions in un-finalized
	// collections cannot be removed from the mempool, as we would want to include
	// such transactions in other forks.
	span, _ = b.tracer.StartSpanFromContext(ctx, trace.COLBuildOnUnfinalizedLookup)
	err = b.populateUnfinalizedAncestryLookup(buildCtx)
	span.End()
	if err != nil {
		return nil, fmt.Errorf("could not populate un-finalized ancestry lookout (parent_id=%x): %w", parentID, err)
	}

	// STEP 1b: create a lookup of all transactions previously included in
	// the finalized collections. Any transactions already included in finalized
	// collections can be removed from the mempool.
	span, _ = b.tracer.StartSpanFromContext(ctx, trace.COLBuildOnFinalizedLookup)
	err = b.populateFinalizedAncestryLookup(buildCtx)
	span.End()
	if err != nil {
		return nil, fmt.Errorf("could not populate finalized ancestry lookup: %w", err)
	}

	// STEP 2: build a payload of valid transactions, while at the same
	// time figuring out the correct reference block ID for the collection.
	span, _ = b.tracer.StartSpanFromContext(ctx, trace.COLBuildOnCreatePayload)
	payload, err := b.buildPayload(buildCtx)
	span.End()
	if err != nil {
		return nil, fmt.Errorf("could not build payload: %w", err)
	}

	// STEP 3: we have a set of transactions that are valid to include on this fork.
	// Now we create the header for the cluster block.
	span, _ = b.tracer.StartSpanFromContext(ctx, trace.COLBuildOnCreateHeader)
	header, err := b.buildHeader(buildCtx, payload, setter, sign)
	span.End()
	if err != nil {
		return nil, fmt.Errorf("could not build header: %w", err)
	}

	proposal := cluster.Block{
		Header:  header,
		Payload: payload,
	}

	// STEP 4: insert the cluster block to the database.
	span, _ = b.tracer.StartSpanFromContext(ctx, trace.COLBuildOnDBInsert)
	err = operation.RetryOnConflict(b.db.Update, procedure.InsertClusterBlock(&proposal))
	span.End()
	if err != nil {
		return nil, fmt.Errorf("could not insert built block: %w", err)
	}

	return proposal.Header, nil
}

// getBlockBuildContext retrieves the required contextual information from the database
// required to build a new block proposal.
// No errors are expected during normal operation.
func (b *Builder) getBlockBuildContext(parentID flow.Identifier) (*blockBuildContext, error) {
	ctx := new(blockBuildContext)
	ctx.config = b.config
	ctx.parentID = parentID
	ctx.lookup = newTransactionLookup()

	var err error
	ctx.parent, err = b.clusterHeaders.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not get parent: %w", err)
	}
	ctx.limiter = newRateLimiter(b.config, ctx.parent.Height+1)

	// retrieve the finalized boundary ON THE CLUSTER CHAIN
	ctx.clusterChainFinalizedBlock, err = b.clusterState.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve cluster chain finalized header: %w", err)
	}

	// retrieve the height and ID of the latest finalized block ON THE MAIN CHAIN
	// this is used as the reference point for transaction expiry
	mainChainFinalizedHeader, err := b.protoState.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve main chain finalized header: %w", err)
	}
	ctx.refChainFinalizedHeight = mainChainFinalizedHeader.Height
	ctx.refChainFinalizedID = mainChainFinalizedHeader.ID()

	// if the epoch has ended and the final block is cached, use the cached values
	if b.epochFinalHeight != nil && b.epochFinalID != nil {
		ctx.refEpochFinalID = b.epochFinalID
		ctx.refEpochFinalHeight = b.epochFinalHeight
		return ctx, nil
	}

	// otherwise, attempt to read them from storage
	err = b.db.View(func(btx *badger.Txn) error {
		var refEpochFinalHeight uint64
		var refEpochFinalID flow.Identifier

		err = operation.RetrieveEpochLastHeight(b.clusterEpoch, &refEpochFinalHeight)(btx)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("unexpected failure to retrieve final height of operating epoch: %w", err)
		}
		err = operation.LookupBlockHeight(refEpochFinalHeight, &refEpochFinalID)(btx)
		if err != nil {
			// if we are able to retrieve the epoch's final height, the block must be finalized
			// therefore failing to look up its height here is an unexpected error
			return irrecoverable.NewExceptionf("could not retrieve ID of finalized final block of operating epoch: %w", err)
		}

		// cache the values
		b.epochFinalHeight = &refEpochFinalHeight
		b.epochFinalID = &refEpochFinalID
		// store the values in the build context
		ctx.refEpochFinalID = b.epochFinalID
		ctx.refEpochFinalHeight = b.epochFinalHeight

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not get block build context: %w", err)
	}
	return ctx, nil
}

// populateUnfinalizedAncestryLookup traverses the unfinalized ancestry backward
// to populate the transaction lookup (used for deduplication) and the rate limiter
// (used to limit transaction submission by payer).
//
// The traversal begins with the block specified by parentID (the block we are
// building on top of) and ends with the oldest unfinalized block in the ancestry.
func (b *Builder) populateUnfinalizedAncestryLookup(ctx *blockBuildContext) error {
	err := fork.TraverseBackward(b.clusterHeaders, ctx.parentID, func(ancestor *flow.Header) error {
		payload, err := b.payloads.ByBlockID(ancestor.ID())
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor payload: %w", err)
		}

		for _, tx := range payload.Collection.Transactions {
			ctx.lookup.addUnfinalizedAncestor(tx.ID())
			ctx.limiter.addAncestor(ancestor.Height, tx)
		}
		return nil
	}, fork.ExcludingHeight(ctx.clusterChainFinalizedBlock.Height))
	return err
}

// populateFinalizedAncestryLookup traverses the reference block height index to
// populate the transaction lookup (used for deduplication) and the rate limiter
// (used to limit transaction submission by payer).
//
// The traversal is structured so that we check every collection whose reference
// block height translates to a possible constituent transaction which could also
// appear in the collection we are building.
func (b *Builder) populateFinalizedAncestryLookup(ctx *blockBuildContext) error {
	minRefHeight := ctx.lowestPossibleReferenceBlockHeight()
	maxRefHeight := ctx.highestPossibleReferenceBlockHeight()
	lookup := ctx.lookup
	limiter := ctx.limiter

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
	//    all its constituent transactions. Hence, for collection C to potentially contain T, it must satisfy c <= t.
	// 2. For T to be eligible for inclusion in collection C, _none_ of the transactions within C are allowed
	// to be expired w.r.t. C's reference block. Hence, for collection C to potentially contain T, it must satisfy t < c + E.
	//
	// Therefore, for collection C to potentially contain transaction T, it must satisfy t - E < c <= t.
	// In other words, we only need to inspect collections with reference block height c ∈ (t-E, t].
	// Consequently, for a set of transactions, with `minRefHeight` (`maxRefHeight`) being the smallest (largest)
	// reference block height, we only need to inspect collections with c ∈ (minRefHeight-E, maxRefHeight].

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

// buildPayload constructs a valid payload based on transactions available in the mempool.
// If the mempool is empty, an empty payload will be returned.
// No errors are expected during normal operation.
func (b *Builder) buildPayload(buildCtx *blockBuildContext) (*cluster.Payload, error) {
	lookup := buildCtx.lookup
	limiter := buildCtx.limiter
	maxRefHeight := buildCtx.highestPossibleReferenceBlockHeight()
	// keep track of the actual smallest reference height of all included transactions
	minRefHeight := maxRefHeight
	minRefID := buildCtx.highestPossibleReferenceBlockID()

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

		// disallow un-finalized reference blocks, and reference blocks beyond the cluster's operating epoch
		if refHeader.Height > maxRefHeight {
			continue
		}

		txID := tx.ID()
		// make sure the reference block is finalized and not orphaned
		blockIDFinalizedAtRefHeight, err := b.mainHeaders.BlockIDByHeight(refHeader.Height)
		if err != nil {
			return nil, fmt.Errorf("could not check that reference block (id=%x) for transaction (id=%x) is finalized: %w", tx.ReferenceBlockID, txID, err)
		}
		if blockIDFinalizedAtRefHeight != tx.ReferenceBlockID {
			// the transaction references an orphaned block - it will never be valid
			b.transactions.Remove(txID)
			continue
		}

		// ensure the reference block is not too old
		if refHeader.Height < buildCtx.lowestPossibleReferenceBlockHeight() {
			// the transaction is expired, it will never be valid
			b.transactions.Remove(txID)
			continue
		}

		// check that the transaction was not already used in un-finalized history
		if lookup.isUnfinalizedAncestor(txID) {
			continue
		}

		// check that the transaction was not already included in finalized history.
		if lookup.isFinalizedAncestor(txID) {
			// remove from mempool, conflicts with finalized block will never be valid
			b.transactions.Remove(txID)
			continue
		}

		// enforce rate limiting rules
		if limiter.shouldRateLimit(tx) {
			if b.config.DryRunRateLimit {
				// log that this transaction would have been rate-limited, but we will still include it in the collection
				b.log.Info().
					Hex("tx_id", logging.ID(txID)).
					Str("payer_addr", tx.Payer.String()).
					Float64("rate_limit", b.config.MaxPayerTransactionRate).
					Msg("dry-run: observed transaction that would have been rate limited")
			} else {
				b.log.Debug().
					Hex("tx_id", logging.ID(txID)).
					Str("payer_addr", tx.Payer.String()).
					Float64("rate_limit", b.config.MaxPayerTransactionRate).
					Msg("transaction is rate-limited")
				continue
			}
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

	// build the payload from the transactions
	payload := cluster.PayloadFromTransactions(minRefID, transactions...)
	return &payload, nil
}

// buildHeader constructs the header for the cluster block being built.
// It invokes the HotStuff setter to set fields related to HotStuff (QC, etc.).
// No errors are expected during normal operation.
func (b *Builder) buildHeader(
	ctx *blockBuildContext,
	payload *cluster.Payload,
	setter func(header *flow.Header) error,
	sign func(*flow.Header) error,
) (*flow.Header, error) {

	header := &flow.Header{
		ChainID:     ctx.parent.ChainID,
		ParentID:    ctx.parentID,
		Height:      ctx.parent.Height + 1,
		PayloadHash: payload.Hash(),
		Timestamp:   time.Now().UTC(),

		// NOTE: we rely on the HotStuff-provided setter to set the other
		// fields, which are related to signatures and HotStuff internals
	}

	// set fields specific to the consensus algorithm
	err := setter(header)
	if err != nil {
		return nil, fmt.Errorf("could not set fields to header: %w", err)
	}
	err = sign(header)
	if err != nil {
		return nil, fmt.Errorf("could not sign proposal: %w", err)
	}
	return header, nil
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

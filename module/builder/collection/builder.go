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

func NewBuilder(
	db *badger.DB,
	tracer module.Tracer,
	mainHeaders storage.Headers,
	clusterHeaders storage.Headers,
	payloads storage.ClusterPayloads,
	transactions mempool.Transactions,
	opts ...Opt,
) *Builder {

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

	return &b
}

// BuildOn creates a new block built on the given parent. It produces a payload
// that is valid with respect to the un-finalized chain it extends.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {
	var proposal cluster.Block

	startTime := time.Now()

	// first we construct a proposal in-memory, ensuring it is a valid extension
	// of chain state -- this can be done in a read-only transaction
	err := b.db.View(func(btx *badger.Txn) error {

		// STEP ONE: Load some things we need to do our work.
		// TODO (ramtin): enable this again
		// b.tracer.StartSpan(parentID, trace.COLBuildOnSetup)
		// defer b.tracer.FinishSpan(parentID, trace.COLBuildOnSetup)

		var parent flow.Header
		err := operation.RetrieveHeader(parentID, &parent)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent: %w", err)
		}

		// retrieve the height and ID of the latest finalized block ON THE MAIN CHAIN
		// this is used as the reference point for transaction expiry
		var refChainFinalizedHeight uint64
		err = operation.RetrieveFinalizedHeight(&refChainFinalizedHeight)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve main finalized height: %w", err)
		}
		var refChainFinalizedID flow.Identifier
		err = operation.LookupBlockHeight(refChainFinalizedHeight, &refChainFinalizedID)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve main finalized ID: %w", err)
		}
		// pre-compute the minimum possible reference block height for transactions
		// included in this collection (actual reference height may be greater)
		minPossibleRefHeight := refChainFinalizedHeight - uint64(flow.DefaultTransactionExpiry-b.config.ExpiryBuffer)
		if minPossibleRefHeight > refChainFinalizedHeight {
			minPossibleRefHeight = 0 // overflow check
		}

		// retrieve the finalized boundary ON THE CLUSTER CHAIN
		var clusterFinal flow.Header
		err = procedure.RetrieveLatestFinalizedClusterHeader(parent.ChainID, &clusterFinal)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve cluster final: %w", err)
		}

		// STEP TWO: create a lookup of all previously used transactions on the
		// part of the chain we care about. We do this separately for
		// un-finalized and finalized sections of the chain to decide whether to
		// remove conflicting transactions from the mempool.

		// TODO (ramtin): enable this again
		// b.tracer.FinishSpan(parentID, trace.COLBuildOnSetup)
		// b.tracer.StartSpan(parentID, trace.COLBuildOnUnfinalizedLookup)
		// defer b.tracer.FinishSpan(parentID, trace.COLBuildOnUnfinalizedLookup)

		// RATE LIMITING: the builder module can be configured to limit the
		// rate at which transactions with a common payer are included in
		// blocks. Depending on the configured limit, we either allow 1
		// transaction every N sequential collections, or we allow K transactions
		// per collection.

		// keep track of transactions in the ancestry to avoid duplicates
		lookup := newTransactionLookup()
		// keep track of transactions to enforce rate limiting
		limiter := newRateLimiter(b.config, parent.Height+1)

		// first, look up previously included transactions in UN-FINALIZED ancestors
		err = b.populateUnfinalizedAncestryLookup(parentID, clusterFinal.Height, lookup, limiter)
		if err != nil {
			return fmt.Errorf("could not populate un-finalized ancestry lookout (parent_id=%x): %w", parentID, err)
		}

		// TODO (ramtin): enable this again
		// b.tracer.FinishSpan(parentID, trace.COLBuildOnUnfinalizedLookup)
		// b.tracer.StartSpan(parentID, trace.COLBuildOnFinalizedLookup)
		// defer b.tracer.FinishSpan(parentID, trace.COLBuildOnFinalizedLookup)

		// second, look up previously included transactions in FINALIZED ancestors
		err = b.populateFinalizedAncestryLookup(minPossibleRefHeight, refChainFinalizedHeight, lookup, limiter)
		if err != nil {
			return fmt.Errorf("could not populate finalized ancestry lookup: %w", err)
		}

		// STEP THREE: build a payload of valid transactions, while at the same
		// time figuring out the correct reference block ID for the collection.

		// TODO (ramtin): enable this again
		// b.tracer.FinishSpan(parentID, trace.COLBuildOnFinalizedLookup)
		// b.tracer.StartSpan(parentID, trace.COLBuildOnCreatePayload)
		// defer b.tracer.FinishSpan(parentID, trace.COLBuildOnCreatePayload)

		// keep track of the actual smallest reference height of all included transactions
		minRefHeight := uint64(math.MaxUint64)
		minRefID := refChainFinalizedID

		var transactions []*flow.TransactionBody
		var totalByteSize uint64
		var totalGas uint64
		for _, tx := range b.transactions.All() {
			fmt.Println("considering transaction for inclusion: ", tx.ID())

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
				return fmt.Errorf("could not retrieve reference header: %w", err)
			}

			// for now, disallow un-finalized reference blocks
			if refChainFinalizedHeight < refHeader.Height {
				continue
			}

			// ensure the reference block is not too old
			txID := tx.ID()
			if refHeader.Height < minPossibleRefHeight {
				fmt.Println("expired tx", txID, refHeader.Height, minPossibleRefHeight)
				// the transaction is expired, it will never be valid
				b.transactions.Rem(txID)
				continue
			}

			// check that the transaction was not already used in un-finalized history
			if lookup.isUnfinalizedAncestor(txID) {
				fmt.Println("confict with unfinalized ancestor: ", txID)
				continue
			}

			// check that the transaction was not already included in finalized history.
			if lookup.isFinalizedAncestor(txID) {
				fmt.Println("confict with finalized ancestor: ", txID)
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

			fmt.Println("adding transaction: ", txID)

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
			return fmt.Errorf("could not set fields to header: %w", err)
		}

		proposal = cluster.Block{
			Header:  &header,
			Payload: &payload,
		}

		// TODO (ramtin): enable this again
		// b.tracer.FinishSpan(parentID, trace.COLBuildOnCreateHeader)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	span, ctx, _ := b.tracer.StartCollectionSpan(context.Background(), proposal.ID(), trace.COLBuildOn, opentracing.StartTime(startTime))
	defer span.Finish()

	dbInsertSpan, _ := b.tracer.StartSpanFromContext(ctx, trace.COLBuildOnDBInsert)
	defer dbInsertSpan.Finish()

	// finally we insert the block in a write transaction
	err = operation.RetryOnConflict(b.db.Update, procedure.InsertClusterBlock(&proposal))
	if err != nil {
		return nil, fmt.Errorf("could not insert built block: %w", err)
	}

	return proposal.Header, err
}

// populateUnfinalizedAncestryLookup traverses the unfinalized ancestry backward
// to populate the transaction lookup (used for deduplication) and the rate limiter
// (used to limit transaction submission by payer).
//
// The traversal begins with the block specified by parentID (the block we are
// building on top of) and ends with the oldest unfinalized block in the ancestry.
func (b *Builder) populateUnfinalizedAncestryLookup(parentID flow.Identifier, finalHeight uint64, lookup *transactionLookup, limiter *rateLimiter) error {

	ancestorID := parentID
	for {
		ancestor, err := b.clusterHeaders.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor header: %w", err)
		}

		if ancestor.Height <= finalHeight {
			break
		}

		payload, err := b.payloads.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor payload: %w", err)
		}

		for _, tx := range payload.Collection.Transactions {
			lookup.addUnfinalizedAncestor(tx.ID())
			limiter.addAncestor(ancestor.Height, tx)
		}
		ancestorID = ancestor.ParentID
	}

	return nil
}

// populateFinalizedAncestryLookup traverses the reference block height index to
// populate the transaction lookup (used for deduplication) and the rate limiter
// (used to limit transaction submission by payer).
//
// The traversal is structured so that we check every collection whose reference
// block height translates to a possible constituent transaction which could also
// appear in the collection we are building.
func (b *Builder) populateFinalizedAncestryLookup(minRefHeight, maxRefHeight uint64, lookup *transactionLookup, limiter *rateLimiter) error {

	// Let E be the global transaction expiry constant, measured in blocks
	//
	// 1. a cluster block's reference block height is equal to the lowest reference
	//    block height of all its constituent transactions
	// 2. a transaction with reference block height T is eligible for inclusion in
	//    any collection with reference block height C where C<=T<C+E
	//
	// Therefore, to guarantee that we find all possible duplicates, we need to check
	// all collections with reference block height in the range (C-E,C+E). Since we
	// know the actual maximum reference height included in this collection, we can
	// further limit the search range to (C-E,maxRefHeight]

	// the finalized cluster blocks which could possibly contain any conflicting transactions
	var clusterBlockIDs []flow.Identifier
	start := minRefHeight - flow.DefaultMaxCollectionByteSize + 1
	end := maxRefHeight
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

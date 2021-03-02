package collection

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dgraph-io/badger/v2"

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
func (builder *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {
	var proposal *cluster.Block

	builder.tracer.StartSpan(parentID, trace.COLBuildOn)
	defer builder.tracer.FinishSpan(parentID, trace.COLBuildOn)

	// first we construct a proposal in-memory, ensuring it is a valid extension
	// of chain state -- this can be done in a read-only transaction
	err := builder.db.View(func(tx *badger.Txn) error {

		// STEP ONE: Load some things we need to do our work.
		builder.tracer.StartSpan(parentID, trace.COLBuildOnSetup)
		defer builder.tracer.FinishSpan(parentID, trace.COLBuildOnSetup)

		var parent flow.Header
		err := operation.RetrieveHeader(parentID, &parent)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent: %w", err)
		}

		// retrieve the height and ID of the latest finalized block ON THE MAIN CHAIN
		// this is used as the reference point for transaction expiry
		var refChainFinalizedHeight uint64
		err = operation.RetrieveFinalizedHeight(&refChainFinalizedHeight)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve main finalized height: %w", err)
		}
		var refChainFinalizedID flow.Identifier
		err = operation.LookupBlockHeight(refChainFinalizedHeight, &refChainFinalizedID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve main finalized ID: %w", err)
		}

		// retrieve the finalized boundary ON THE CLUSTER CHAIN
		var clusterFinal flow.Header
		err = procedure.RetrieveLatestFinalizedClusterHeader(parent.ChainID, &clusterFinal)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve cluster final: %w", err)
		}

		//// short-circuit and build an empty collection if the number of available
		//// transactions isn't enough to satisfy min collection size
		//if builder.transactions.Size() < builder.config.MinCollectionSize {
		//	payload := cluster.EmptyPayload(refChainFinalizedID)
		//	proposal, err = builder.createProposal(&parent, parentID, &payload, setter)
		//	if err != nil {
		//		return fmt.Errorf("could not create empty proposal: %w", err)
		//	}
		//	return nil
		//}

		// STEP TWO: create a lookup of all previously used transactions on the
		// part of the chain we care about. We do this separately for
		// un-finalized and finalized sections of the chain to decide whether to
		// remove conflicting transactions from the mempool.

		builder.tracer.FinishSpan(parentID, trace.COLBuildOnSetup)
		builder.tracer.StartSpan(parentID, trace.COLBuildOnUnfinalizedLookup)
		defer builder.tracer.FinishSpan(parentID, trace.COLBuildOnUnfinalizedLookup)

		// RATE LIMITING: the builder module can be configured to limit the
		// rate at which transactions with a common payer are included in
		// blocks. Depending on the configured limit, we either allow 1
		// transaction every N sequential collections, or we allow K transactions
		// per collection.

		// keep track of number of empty ancestors to skip min size requirement
		numEmptyAncestors := 0
		// keep track of transactions in the ancestry to avoid duplicates
		lookup := newTransactionLookup()
		// keep track of transactions to enforce rate limiting
		limiter := newRateLimiter(builder.config, parent.Height+1)

		// look up previously included transactions in UN-FINALIZED ancestors
		ancestorID := parentID
		clusterFinalID := clusterFinal.ID()
		for ancestorID != clusterFinalID {
			ancestor, err := builder.clusterHeaders.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
			}

			if ancestor.Height <= clusterFinal.Height {
				return fmt.Errorf("should always build on last finalized block")
			}

			payload, err := builder.payloads.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
			}

			collection := payload.Collection
			for _, tx := range collection.Transactions {
				lookup.addUnfinalizedAncestor(tx.ID())
				limiter.addAncestor(ancestor.Height, tx)
			}

			if len(collection.Transactions) == 0 && len(lookup.unfinalized) == 0 {
				numEmptyAncestors++
			}

			ancestorID = ancestor.ParentID
		}

		builder.tracer.FinishSpan(parentID, trace.COLBuildOnUnfinalizedLookup)
		builder.tracer.StartSpan(parentID, trace.COLBuildOnFinalizedLookup)
		defer builder.tracer.FinishSpan(parentID, trace.COLBuildOnFinalizedLookup)

		//TODO for now we check a fixed # of finalized ancestors - we should
		// instead look back based on reference block ID and expiry
		// ref: https://github.com/dapperlabs/flow-go/issues/3556
		limit := clusterFinal.Height - flow.DefaultTransactionExpiry
		if limit > clusterFinal.Height { // overflow check
			limit = 0
		}

		// look up previously included transactions in FINALIZED ancestors
		ancestorID = clusterFinal.ID()
		ancestorHeight := clusterFinal.Height
		for ancestorHeight > limit {
			ancestor, err := builder.clusterHeaders.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
			}
			payload, err := builder.payloads.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
			}

			collection := payload.Collection
			for _, tx := range collection.Transactions {
				lookup.addFinalizedAncestor(tx.ID())
				limiter.addAncestor(ancestor.Height, tx)
			}

			if len(collection.Transactions) == 0 && lookup.size() == 0 {
				numEmptyAncestors++
			}

			ancestorID = ancestor.ParentID
			ancestorHeight = ancestor.Height
		}

		// STEP THREE: build a payload of valid transactions, while at the same
		// time figuring out the correct reference block ID for the collection.
		builder.tracer.FinishSpan(parentID, trace.COLBuildOnFinalizedLookup)
		builder.tracer.StartSpan(parentID, trace.COLBuildOnCreatePayload)
		defer builder.tracer.FinishSpan(parentID, trace.COLBuildOnCreatePayload)

		minRefHeight := uint64(math.MaxUint64)
		// start with the finalized reference ID (longest expiry time)
		minRefID := refChainFinalizedID

		var transactions []*flow.TransactionBody
		var totalByteSize uint64
		var totalGas uint64
		for _, tx := range builder.transactions.All() {

			// if we have reached maximum number of transactions, stop
			if uint(len(transactions)) >= builder.config.MaxCollectionSize {
				break
			}

			txByteSize := uint64(tx.ByteSize())
			// ignore transactions with tx byte size bigger that the max amount per collection
			// this case shouldn't happen ever since we keep a limit on tx byte size but in case
			// we keep this condition
			if txByteSize > builder.config.MaxCollectionByteSize {
				continue
			}

			// because the max byte size per tx is way smaller than the max collection byte size, we can stop here and not continue.
			// to make it more effective in the future we can continue adding smaller ones
			if totalByteSize+txByteSize > builder.config.MaxCollectionByteSize {
				break
			}

			// ignore transactions with max gas bigger that the max total gas per collection
			// this case shouldn't happen ever but in case we keep this condition
			if tx.GasLimit > builder.config.MaxCollectionTotalGas {
				continue
			}

			// cause the max gas limit per tx is way smaller than the total max gas per collection, we can stop here and not continue.
			// to make it more effective in the future we can continue adding smaller ones
			if totalGas+tx.GasLimit > builder.config.MaxCollectionTotalGas {
				break
			}

			// retrieve the main chain header that was used as reference
			refHeader, err := builder.mainHeaders.ByBlockID(tx.ReferenceBlockID)
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
			if refChainFinalizedHeight-refHeader.Height > uint64(flow.DefaultTransactionExpiry-builder.config.ExpiryBuffer) {
				// the transaction is expired, it will never be valid
				builder.transactions.Rem(txID)
				continue
			}

			// check that the transaction was not already used in un-finalized history
			if lookup.isUnfinalizedAncestor(txID) {
				continue
			}

			// check that the transaction was not already included in finalized history.
			if lookup.isFinalizedAncestor(txID) {
				// remove from mempool, conflicts with finalized block will never be valid
				builder.transactions.Rem(txID)
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
		builder.tracer.FinishSpan(parentID, trace.COLBuildOnCreatePayload)
		builder.tracer.StartSpan(parentID, trace.COLBuildOnCreateHeader)
		defer builder.tracer.FinishSpan(parentID, trace.COLBuildOnCreateHeader)

		var payload cluster.Payload
		if uint(len(transactions)) >= builder.config.MinCollectionSize || numEmptyAncestors > 10 {
			// if we have enough transactions form them into a collection
			payload = cluster.PayloadFromTransactions(minRefID, transactions...)
		} else {
			// if we haven't gathered the minimum number of transactions, create
			// an empty collection with the maximum reference block
			payload = cluster.EmptyPayload(minRefID)
		}

		proposal, err = builder.createProposal(&parent, parentID, &payload, setter)
		if err != nil {
			return fmt.Errorf("could not create proposal: %w", err)
		}

		builder.tracer.FinishSpan(parentID, trace.COLBuildOnCreateHeader)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	builder.tracer.StartSpan(parentID, trace.COLBuildOnDBInsert)
	defer builder.tracer.FinishSpan(parentID, trace.COLBuildOnDBInsert)

	// SANITY CHECK: ensure the proposal was correctly set
	if proposal == nil {
		return nil, fmt.Errorf("attempting to propose nil proposal")
	}

	// finally we insert the block in a write transaction
	err = operation.RetryOnConflict(builder.db.Update, procedure.InsertClusterBlock(proposal))
	if err != nil {
		return nil, fmt.Errorf("could not insert built block: %w", err)
	}

	return proposal.Header, err
}

func (builder *Builder) createProposal(
	parent *flow.Header,
	parentID flow.Identifier,
	payload *cluster.Payload,
	setter func(*flow.Header) error,
) (*cluster.Block, error) {

	header := &flow.Header{
		ChainID:     parent.ChainID,
		ParentID:    parentID,
		Height:      parent.Height + 1,
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

	block := &cluster.Block{
		Header:  header,
		Payload: payload,
	}
	return block, nil
}

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
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {
	var proposal cluster.Block

	b.tracer.StartSpan(parentID, trace.COLBuildOn)
	defer b.tracer.FinishSpan(parentID, trace.COLBuildOn)

	// first we construct a proposal in-memory, ensuring it is a valid extension
	// of chain state -- this can be done in a read-only transaction
	err := b.db.View(func(tx *badger.Txn) error {

		// STEP ONE: Load some things we need to do our work.
		b.tracer.StartSpan(parentID, trace.COLBuildOnSetup)
		defer b.tracer.FinishSpan(parentID, trace.COLBuildOnSetup)

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

		// STEP TWO: create a lookup of all previously used transactions on the
		// part of the chain we care about. We do this separately for
		// un-finalized and finalized sections of the chain to decide whether to
		// remove conflicting transactions from the mempool.

		b.tracer.FinishSpan(parentID, trace.COLBuildOnSetup)
		b.tracer.StartSpan(parentID, trace.COLBuildOnUnfinalizedLookup)
		defer b.tracer.FinishSpan(parentID, trace.COLBuildOnUnfinalizedLookup)

		// RATE LIMITING: the builder module can be configured to limit the
		// rate at which transactions with a common payer are included in
		// blocks. Depending on the configured limit, we either allow 1
		// transaction every N sequential collections, or we allow K transactions
		// per collection.

		// keep track of transactions in the ancestry to avoid duplicates
		lookup := newTxLookup()
		// keep track of transactions to enforce rate limiting
		limiter := newRateLimiter(b.config, parent.Height+1)

		// look up previously included transactions in UN-FINALIZED ancestors
		ancestorID := parentID
		clusterFinalID := clusterFinal.ID()
		for ancestorID != clusterFinalID {
			ancestor, err := b.clusterHeaders.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get noteAncestor header (%x): %w", ancestorID, err)
			}

			if ancestor.Height <= clusterFinal.Height {
				return fmt.Errorf("should always build on last finalized block")
			}

			payload, err := b.payloads.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get noteAncestor payload (%x): %w", ancestorID, err)
			}

			collection := payload.Collection
			for _, tx := range collection.Transactions {
				lookup.addUnfinalizedAncestor(tx.ID())
				limiter.addAncestor(ancestor.Height, tx)
			}
			ancestorID = ancestor.ParentID
		}

		b.tracer.FinishSpan(parentID, trace.COLBuildOnUnfinalizedLookup)
		b.tracer.StartSpan(parentID, trace.COLBuildOnFinalizedLookup)
		defer b.tracer.FinishSpan(parentID, trace.COLBuildOnFinalizedLookup)

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
			ancestor, err := b.clusterHeaders.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get noteAncestor header (%x): %w", ancestorID, err)
			}
			payload, err := b.payloads.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get noteAncestor payload (%x): %w", ancestorID, err)
			}

			collection := payload.Collection
			for _, tx := range collection.Transactions {
				lookup.addFinalizedAncestor(tx.ID())
				limiter.addAncestor(ancestor.Height, tx)
			}

			ancestorID = ancestor.ParentID
			ancestorHeight = ancestor.Height
		}

		// STEP THREE: build a payload of valid transactions, while at the same
		// time figuring out the correct reference block ID for the collection.
		b.tracer.FinishSpan(parentID, trace.COLBuildOnFinalizedLookup)
		b.tracer.StartSpan(parentID, trace.COLBuildOnCreatePayload)
		defer b.tracer.FinishSpan(parentID, trace.COLBuildOnCreatePayload)

		minRefHeight := uint64(math.MaxUint64)
		// start with the finalized reference ID (longest expiry time)
		minRefID := refChainFinalizedID

		var transactions []*flow.TransactionBody
		for _, tx := range b.transactions.All() {

			// if we have reached maximum number of transactions, stop
			if uint(len(transactions)) >= b.config.MaxCollectionSize {
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
			if refChainFinalizedHeight-refHeader.Height > uint64(flow.DefaultTransactionExpiry-b.config.ExpiryBuffer) {
				// the transaction is expired, it will never be valid
				b.transactions.Rem(txID)
				continue
			}

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
		}

		// STEP FOUR: we have a set of transactions that are valid to include
		// on this fork. Now we need to create the collection that will be
		// used in the payload and construct the final proposal model
		b.tracer.FinishSpan(parentID, trace.COLBuildOnCreatePayload)
		b.tracer.StartSpan(parentID, trace.COLBuildOnCreateHeader)
		defer b.tracer.FinishSpan(parentID, trace.COLBuildOnCreateHeader)

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

		b.tracer.FinishSpan(parentID, trace.COLBuildOnCreateHeader)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	b.tracer.StartSpan(parentID, trace.COLBuildOnDBInsert)
	defer b.tracer.FinishSpan(parentID, trace.COLBuildOnDBInsert)

	// finally we insert the block in a write transaction
	err = operation.RetryOnConflict(b.db.Update, procedure.InsertClusterBlock(&proposal))
	if err != nil {
		return nil, fmt.Errorf("could not insert built block: %w", err)
	}

	return proposal.Header, err
}

func (b *Builder) isUnlimitedPayer(payer flow.Address) bool {
	_, exists := b.config.UnlimitedPayers[payer]
	return exists
}

type txlookup struct {
	finalized   map[flow.Identifier]struct{}
	unfinalized map[flow.Identifier]struct{}
}

func newTxLookup() *txlookup {
	lookup := &txlookup{
		finalized:   make(map[flow.Identifier]struct{}),
		unfinalized: make(map[flow.Identifier]struct{}),
	}
	return lookup
}

// note the existence of a transaction in a finalized noteAncestor collection
func (lookup *txlookup) addFinalizedAncestor(txID flow.Identifier) {
	lookup.finalized[txID] = struct{}{}
}

// note the existence of a transaction in a unfinalized noteAncestor collection
func (lookup *txlookup) addUnfinalizedAncestor(txID flow.Identifier) {
	lookup.unfinalized[txID] = struct{}{}
}

// checks whether the given transaction ID is in a finalized noteAncestor collection
func (lookup *txlookup) isFinalizedAncestor(txID flow.Identifier) bool {
	_, exists := lookup.finalized[txID]
	return exists
}

// checks whether the given transaction ID is in a unfinalized noteAncestor collection
func (lookup *txlookup) isUnfinalizedAncestor(txID flow.Identifier) bool {
	_, exists := lookup.unfinalized[txID]
	return exists
}

// implements rate limiting by payer address.
type ratelimiter struct {

	// maximum rate of transactions/payer/collection (from Config)
	rate float64
	// set of unlimited payer address (from Config)
	unlimited map[flow.Address]struct{}
	// height of the collection we are building
	height uint64

	// for each payer, height of latest collection in which a transaction for
	// which they were payer was included
	latestCollectionHeight map[flow.Address]uint64
	// number of transactions included in the currently built block per payer
	txIncludedCount map[flow.Address]uint
}

func newRateLimiter(conf Config, height uint64) *ratelimiter {
	limiter := &ratelimiter{
		rate:                   conf.MaxPayerTransactionRate,
		unlimited:              conf.UnlimitedPayers,
		height:                 height,
		latestCollectionHeight: make(map[flow.Address]uint64),
		txIncludedCount:        make(map[flow.Address]uint),
	}
	return limiter
}

// note the existence and height of a transaction in an ancestor collection.
func (limiter *ratelimiter) addAncestor(height uint64, tx *flow.TransactionBody) {

	// skip tracking payers if we aren't rate-limiting or are configured
	// to allow multiple transactions per payer per collection
	if limiter.rate >= 1 || limiter.rate <= 0 {
		return
	}

	latest := limiter.latestCollectionHeight[tx.Payer]
	if height >= latest {
		limiter.latestCollectionHeight[tx.Payer] = height
	}
}

// note that we have added a transaction to the collection under construction.
func (limiter *ratelimiter) transactionIncluded(tx *flow.TransactionBody) {
	limiter.txIncludedCount[tx.Payer]++
}

// applies the rate limiting rules, returning whether the transaction should be
// omitted from the collection under construction.
func (limiter *ratelimiter) shouldRateLimit(tx *flow.TransactionBody) bool {

	payer := tx.Payer

	// skip rate limiting if it is turned off or the payer is unlimited
	_, isUnlimited := limiter.unlimited[payer]
	if limiter.rate == 0 || isUnlimited {
		return false
	}

	// if rate >=1, we only consider the current collection and rate limit once
	// the number of transactions for the payer exceeds rate
	if limiter.rate >= 1 {
		if limiter.txIncludedCount[payer] >= uint(math.Floor(limiter.rate)) {
			return true
		}
	}

	// if rate < 1, we need to look back to see when a transaction by this payer
	// was most recently included - we rate limit if the # of collections since
	// the payer's last transaction is less than ceil(1/rate)
	if limiter.rate < 1 {

		// rate limit if we've already include a transaction for this payer, we allow
		// AT MOST one transaction per payer in a given collection
		if limiter.txIncludedCount[payer] > 0 {
			return true
		}

		// otherwise, check whether sufficiently many empty collection
		// have been built since the last transaction from the payer

		latestHeight, hasLatest := limiter.latestCollectionHeight[payer]
		// if there is no recent transaction, don't rate limit
		if !hasLatest {
			return false
		}

		if limiter.height-latestHeight < uint64(math.Ceil(1/limiter.rate)) {
			return true
		}
	}

	return false
}

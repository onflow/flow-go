package collection

import (
	"fmt"
	"math"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

type config struct {
	maxCollectionSize uint // TODO not used
	// the number of blocks we subtract from the expiry when deciding whether a
	// transaction has expired -- this describes how many main chain blocks can
	// be built *without* this collection before it expires, in the worst case.
	expiryBuffer uint64
}

var defaultConfig = config{
	maxCollectionSize: 1000, // TODO not used
	expiryBuffer:      10,
}

// Builder is the builder for collection block payloads. Upon providing a
// payload hash, it also memorizes the payload contents.
//
// NOTE: Builder is NOT safe for use with multiple goroutines. Since the
// HotStuff event loop is the only consumer of this interface and is single
// threaded, this is OK.
type Builder struct {
	db           *badger.DB
	transactions mempool.Transactions

	// cache of block ID -> height for checking transaction expiry
	// NOTE: these are blocks from the main consensus chain, NOT from the cluster
	cache map[flow.Identifier]uint64

	conf config
}

func NewBuilder(db *badger.DB, transactions mempool.Transactions) *Builder {
	b := &Builder{
		db:           db,
		transactions: transactions,
		cache:        make(map[flow.Identifier]uint64),
		conf:         defaultConfig,
	}
	return b
}

// BuildOn creates a new block built on the given parent. It produces a payload
// that is valid with respect to the un-finalized chain it extends.
func (builder *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header)) (*flow.Header, error) {
	var header *flow.Header
	err := builder.db.Update(func(tx *badger.Txn) error {

		// retrieve a set of non-expired transactions from the mempool
		candidateTxIDs, err := builder.getCandidateTransactions(tx)
		if err != nil {
			return fmt.Errorf("could not get candidate transactions: %w", err)
		}

		// retrieve the parent to set the height and have chain ID
		var parent flow.Header
		err = operation.RetrieveHeader(parentID, &parent)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent: %w", err)
		}

		// find any transactions that conflict with finalized or un-finalized
		// ancestors of the block we are building
		//TODO currently the distance we look back in the payload to de-duplicate
		// transactions is hard-coded to double the transaction expiry constant.
		// Since cluster consensus should run at roughly the same rate as main
		// consensus, this will catch most duplicates. However, to guarantee no
		// duplicates we need to create an index mapping cluster block heights
		// to reference block heights.
		// For now, this heuristic is acceptable, since duplicate transactions
		// will not be executed by EXE nodes.
		var invalidIDs map[flow.Identifier]struct{}
		err = operation.CheckCollectionPayload(parent.Height, parent.ID(), candidateTxIDs, &invalidIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not check collection payload: %w", err)
		}

		// keep track of lowest reference block ID - this will be the reference
		// block ID for the collection as a whole
		var (
			colRefID     flow.Identifier
			minRefHeight uint64 = math.MaxUint64
		)

		// populate the final list of transaction IDs for the block - these
		// are guaranteed to be valid
		var validTransactions []*flow.TransactionBody
		for _, txID := range candidateTxIDs {

			_, isInvalid := invalidIDs[txID]
			if isInvalid {
				// remove from mempool, it will never be valid
				builder.transactions.Rem(txID)
				continue
			}

			// add ONLY non-conflicting transaction to the final payload
			nextTx, err := builder.transactions.ByID(txID)
			if err != nil {
				return fmt.Errorf("could not get transaction: %w", err)
			}

			height, ok := builder.cache[nextTx.ReferenceBlockID]
			// this should never happen, since we populated the cache with all
			// these in getCandidateTransactions
			if !ok {
				return fmt.Errorf("could not check reference height")
			}

			// ensure we find the lowest reference block height
			if height < minRefHeight {
				minRefHeight = height
				colRefID = nextTx.ReferenceBlockID
			}

			validTransactions = append(validTransactions, nextTx)
		}

		// build the payload from the transactions
		payload := cluster.PayloadFromTransactions(colRefID, validTransactions...)

		header = &flow.Header{
			ChainID:     parent.ChainID,
			ParentID:    parentID,
			Height:      parent.Height + 1,
			PayloadHash: payload.Hash(),
			Timestamp:   time.Now().UTC(),

			// the following fields should be set by the provided setter function
			View:           0,
			ParentVoterIDs: nil,
			ParentVoterSig: nil,
			ProposerID:     flow.ZeroID,
			ProposerSig:    nil,
		}

		// set fields specific to the consensus algorithm
		setter(header)

		// insert the header for the newly built block
		err = operation.InsertHeader(header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert cluster header: %w", err)
		}

		// insert the payload
		// this inserts the collection AND all constituent transactions
		err = procedure.InsertClusterPayload(header, &payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert cluster payload: %w", err)
		}

		// index the payload by block ID
		err = procedure.IndexClusterPayload(header, &payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index cluster payload: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	return header, nil
}

// getCandidateTransactions creates a set of candidate transactions from the
// mempool. It guarantees that all candidates are not expired and do not
// reference un-finalized blocks.
func (builder *Builder) getCandidateTransactions(tx *badger.Txn) ([]flow.Identifier, error) {

	// retrieve the finalized head ON THE MAIN CHAIN in order to know which
	// transactions have expired and should be discarded
	var final flow.Header
	err := procedure.RetrieveLatestFinalizedHeader(&final)(tx)

	// build a payload that includes as many transactions from the memory
	// that have not expired
	// TODO make empty collections / limit size based on collection min/max size constraints
	var candidateTxIDs []flow.Identifier
	for _, candidateTx := range builder.transactions.All() {

		candidateID := candidateTx.ID()
		refID := candidateTx.ReferenceBlockID

		refHeight, cached := builder.cache[refID]
		// the block isn't in our cache, retrieve it from storage
		if !cached {
			var ref flow.Header
			err = operation.RetrieveHeader(refID, &ref)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve reference block: %w", err)
			}

			// sanity check: ensure the reference block is from the main chain
			if ref.ChainID != flow.DefaultChainID {
				return nil, fmt.Errorf("invalid reference block (chain_id=%s)", ref.ChainID)
			}

			refHeight = ref.Height
			builder.cache[refID] = ref.Height
		}

		// for now, disallow un-finalized reference blocks
		if final.Height < refHeight {
			continue
		}

		// ensure the reference block is not too old
		if final.Height-refHeight > flow.DefaultTransactionExpiry-builder.conf.expiryBuffer {
			// the transaction is expired, it will never be valid
			builder.transactions.Rem(candidateID)
			continue
		}

		candidateTxIDs = append(candidateTxIDs, candidateTx.ID())
	}

	// invalidate expired items in reference block ID cache
	// NOTE: the maximum number of items here is 100s, so this linear-time
	// invalidation should be OK
	for id, height := range builder.cache {
		if final.Height-height > flow.DefaultTransactionExpiry {
			delete(builder.cache, id)
		}
	}

	return candidateTxIDs, nil
}

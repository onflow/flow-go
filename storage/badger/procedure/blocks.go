// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"
	"math"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// InsertBlock inserts a block to the storage
func InsertBlock(block *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// store the block header
		err := operation.InsertHeader(&block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert block header: %w", err)
		}

		// insert the block payload
		err = InsertPayload(&block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert block payload: %w", err)
		}

		// index the block payload
		err = IndexPayload(&block.Header, &block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index block payload: %w", err)
		}

		return nil

	}
}

// RetrieveBlock retrieves a block by the given blockID
func RetrieveBlock(blockID flow.Identifier, block *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// get the block header
		err := operation.RetrieveHeader(blockID, &block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// get the block payload
		err = RetrievePayload(block.Header.ID(), &block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve payload: %w", err)
		}

		return nil
	}
}

// RetrieveUnfinalizedAncestors retrieves all un-finalized ancestors of the
// given block, including the block itself, reverse-ordered by height.
func RetrieveUnfinalizedAncestors(blockID flow.Identifier, unfinalized *[]*flow.Header) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the block header of the block we want to finalize
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// retrieve the current boundary of the finalized state
		var boundary uint64
		err = operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// if the block has already been finalized, exit early
		if boundary >= header.Height {
			*unfinalized = []*flow.Header{}
			return nil
		}

		// retrieve the ID of the last finalized block as marker for stopping
		var headID flow.Identifier
		err = operation.RetrieveNumber(boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// in order to validate the validity of all changes, we need to iterate
		// through the blocks that need to be finalized from oldest to youngest;
		// we thus start at the youngest remember all of the intermediary steps
		// while tracing back until we reach the finalized state
		*unfinalized = append(*unfinalized, &header)
		parentID := header.ParentID
		for parentID != headID {
			var parent flow.Header
			err = operation.RetrieveHeader(parentID, &parent)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve parent (%x): %w", parentID, err)
			}
			*unfinalized = append(*unfinalized, &parent)
			parentID = parent.ParentID
		}

		return nil
	}
}

// RetrieveUnfinalizedDescendants find all unfinalized block IDs that connect to the finalized block
func RetrieveUnfinalizedDescendants(unfinalizedBlockIDs *[]flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		var boundary uint64
		// retrieve the current finalized view
		err := operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// retrieve the block ID of the last finalized block
		var headID flow.Identifier
		err = operation.RetrieveNumber(boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// find all the unfinalized blocks that connect to the finalized block
		// the order guarantees that if a block requires certain blocks to connect to the
		// finalized block, those connecting blocks must appear before this block.
		operation.FindDescendants(boundary, headID, unfinalizedBlockIDs)

		return nil
	}
}

// FinalizeBlock finalizes the block by the given blockID and all blocks along the path
// that connects to the finalized block.
func FinalizeBlock(blockID flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the header to check the parent
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		// retrieve the current finalized state boundary
		var boundary uint64
		err = operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// retrieve the ID of the boundary head
		var headID flow.Identifier
		err = operation.RetrieveNumber(boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// check that the head ID is the parent of the block we finalize
		if header.ParentID != headID {
			return fmt.Errorf("can't finalize non-child of chain head")
		}

		// insert the number to block mapping
		err = operation.InsertNumber(header.Height, header.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert number mapping: %w", err)
		}

		// update the finalized boundary
		err = operation.UpdateBoundary(header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not update finalized boundary: %w", err)
		}

		// NOTE: we don't want to prune forks that have become invalid here, so
		// that we can keep validating entities and generating slashing
		// challenges for some time - the pruning should happen some place else
		// after a certain delay of blocks

		return nil
	}
}

// Bootstrap inserts the genesis block to the storage
func Bootstrap(commit flow.StateCommitment, genesis *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// insert the block header & payload
		err := InsertBlock(genesis)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block: %w", err)
		}

		// apply the stake deltas
		err = ApplyDeltas(genesis.Height, genesis.Identities)(tx)
		if err != nil {
			return fmt.Errorf("could not apply stake deltas: %w", err)
		}

		// generate genesis execution result
		result := flow.ExecutionResult{ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: flow.ZeroID,
			BlockID:          genesis.ID(),
			FinalStateCommit: commit,
		}}

		// generate genesis block seal
		seal := flow.Seal{
			BlockID:      genesis.ID(),
			ResultID:     result.ID(),
			InitialState: flow.GenesisStateCommitment,
			FinalState:   result.FinalStateCommit,
		}

		// insert genesis block seal
		err = operation.InsertSeal(&seal)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis seal: %w", err)
		}

		// index genesis block seal
		err = operation.IndexSealIDByBlock(genesis.ID(), seal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index genesis seal: %w", err)
		}

		// index the state commitment for the void state (before genesis)
		err = operation.IndexStateCommitment(flow.ZeroID, seal.InitialState)(tx)
		if err != nil {
			return fmt.Errorf("could not index void commit: %w", err)
		}

		// index the genesis seal state commitment (after genesis)
		err = operation.IndexStateCommitment(genesis.ID(), seal.FinalState)(tx)
		if err != nil {
			return fmt.Errorf("could not index genesis commit: %w", err)
		}

		// insert first execution result
		err = operation.InsertExecutionResult(&result)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis result: %w", err)
		}

		// index first execution block for genesis block
		err = operation.IndexExecutionResult(genesis.ID(), result.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index genesis result: %w", err)
		}

		// insert the block number mapping
		err = operation.InsertNumber(0, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not initialize boundary: %w", err)
		}

		// insert the finalized boundary
		err = operation.InsertBoundary(genesis.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not update boundary: %w", err)
		}

		return nil
	}
}
func IndexBlockByGuarantees(blockID flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		block := &flow.Block{}
		err := RetrieveBlock(blockID, block)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block for guarantee index: %w", err)
		}

		for _, g := range block.Payload.Guarantees {
			collectionID := g.CollectionID
			err = operation.IndexHeaderByCollection(collectionID, block.Header.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not add block guarantee index: %w", err)
			}
		}
		return nil
	}
}

func RetrieveBlockByCollectionID(collectionID flow.Identifier, block *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		headerID := &flow.Identifier{}

		// get the block header
		err := operation.LookupHeaderIDByCollectionID(collectionID, headerID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// get the complete block
		err = RetrieveBlock(*headerID, block)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}
		return nil
	}
}

func RetrieveLatestFinalizedHeader(header *flow.Header) func(tx *badger.Txn) error {
	var number uint64 = math.MaxUint64
	blockID := flow.ZeroID
	return RetrieveHeader(&number, &blockID, header)
}

func RetrieveHeader(number *uint64, blockID *flow.Identifier, header *flow.Header) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {

		// set the number to boundary if it's at max uint64
		if *number == math.MaxUint64 {
			err := operation.RetrieveBoundary(number)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve boundary: %w", err)
			}
		}

		// check if hash is nil and try to get it from height
		if *blockID == flow.ZeroID {
			err := operation.RetrieveNumber(*number, blockID)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve hash (%d): %w", number, err)
			}
		}

		// get the height for our desired target hash
		err := operation.RetrieveHeader(*blockID, header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header (%x): %w", blockID, err)
		}

		return nil
	}
}

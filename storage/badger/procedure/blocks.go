// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// InsertBlock inserts a block to the storage
func InsertBlock(blockID flow.Identifier, block *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// store the block header
		err := operation.InsertHeader(blockID, block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert block header: %w", err)
		}

		// insert the block payload
		err = InsertPayload(blockID, block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert block payload: %w", err)
		}

		return nil

	}
}

// RetrieveBlock retrieves a block by the given blockID
func RetrieveBlock(blockID flow.Identifier, block *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// get the block header
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// get the block payload
		var payload flow.Payload
		err = RetrievePayload(blockID, &payload)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve payload: %w", err)
		}

		// build block and replace original
		*block = flow.Block{
			Header:  &header,
			Payload: &payload,
		}

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
		var finalized uint64
		err = operation.RetrieveFinalizedHeight(&finalized)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized height: %w", err)
		}

		// retrieve the ID of the boundary head
		var finalID flow.Identifier
		err = operation.LookupBlockHeight(finalized, &finalID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// check that the head ID is the parent of the block we finalize
		if header.ParentID != finalID {
			return fmt.Errorf("can't finalize non-child of chain head")
		}

		// retrieve the parent of the extension header
		var final flow.Header
		err = operation.RetrieveHeader(finalID, &final)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve final header: %w", err)
		}

		// check that the height is exactly on higher
		if header.Height != final.Height+1 {
			return fmt.Errorf("extension has invalid height (extension: %d, final: %d)", header.Height, final.Height)
		}

		// insert the number to block mapping
		err = operation.IndexBlockHeight(header.Height, header.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert number mapping: %w", err)
		}

		// update the finalized boundary
		err = operation.UpdateFinalizedHeight(header.Height)(tx)
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

		// insert genesis identities
		err := operation.InsertIdentities(genesis.Payload.Identities)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis identities: %w", err)
		}

		// insert the block header
		genesisID := genesis.ID()
		err = operation.InsertHeader(genesisID, genesis.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert header: %w", err)
		}

		// NOTE: no need to insert the payload, both seal and guarantees should
		// be empty and we don't want guarantees inserted as the payload, just
		// as the genesis identities

		// index the genesis payload (still useful to have empty index entry)
		err = InsertPayload(genesisID, genesis.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index payload: %w", err)
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
		err = operation.InsertSeal(seal.ID(), &seal)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis seal: %w", err)
		}

		// index genesis block seal
		err = operation.IndexBlockSeal(genesis.ID(), seal.ID())(tx)
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
		err = operation.IndexBlockHeight(0, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not initialize boundary: %w", err)
		}

		// insert last started view
		err = operation.InsertStartedView(genesis.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}

		// insert last voted view
		err = operation.InsertVotedView(genesis.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}

		// insert the finalized boundary
		err = operation.InsertFinalizedHeight(genesis.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert finalized height: %w", err)
		}

		// insert the executed boundary
		err = operation.InsertExecutedHeight(genesis.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert executed height: %w", err)
		}

		// insert the sealed boundary
		err = operation.InsertSealedHeight(genesis.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert sealed height: %w", err)
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

package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(genesis *cluster.Block) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// check chain ID
		if genesis.ChainID != m.state.chainID {
			return fmt.Errorf("genesis chain ID (%s) does not match configured (%s)", genesis.ChainID, m.state.chainID)
		}

		// check header number
		if genesis.View != 0 {
			return fmt.Errorf("genesis number should be 0 (got %d)", genesis.View)
		}

		// check header parent ID
		if genesis.ParentID != flow.ZeroID {
			return fmt.Errorf("genesis parent ID must be zero hash (got %x)", genesis.ParentID)
		}

		// check payload
		collSize := len(genesis.Collection.Transactions)
		if collSize != 0 {
			return fmt.Errorf("genesis collection should contain no transactions (got %d)", collSize)
		}

		// insert block payload
		err := operation.InsertCollection(&genesis.Payload.Collection)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block payload: %w", err)
		}

		// insert block
		err = procedure.InsertClusterBlock(genesis)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block: %w", err)
		}

		// insert block number -> ID mapping
		err = operation.InsertNumberForCluster(genesis.ChainID, genesis.View, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis number: %w", err)
		}

		// insert boundary
		err = operation.InsertBoundaryForCluster(genesis.ChainID, genesis.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis boundary: %w", err)
		}

		return nil
	})
}

func (m *Mutator) Extend(blockID flow.Identifier) error {
	return m.state.db.View(func(tx *badger.Txn) error {

		// retrieve the block
		var block cluster.Block
		err := procedure.RetrieveClusterBlock(blockID, &block)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		// check chain ID
		if block.ChainID != m.state.chainID {
			return fmt.Errorf("new block chain ID (%s) does not match configured (%s)", block.ChainID, m.state.chainID)
		}

		// check for duplicate transactions in block's ancestry
		parentHeight := block.Height - 1
		parentID := block.ParentID
		err = operation.VerifyCollectionPayload(parentHeight, parentID, block.Payload.Collection.Transactions)(tx)
		if errors.Is(err, storage.ErrAlreadyIndexed) {
			return fmt.Errorf("found duplicate transaction in payload: %w", err)
		}
		if err != nil {
			return fmt.Errorf("could not verify collection payload: %w", err)
		}

		// get the chain ID, which determines which cluster state to query
		chainID := block.ChainID

		// get finalized state boundary
		var boundary uint64
		err = operation.RetrieveBoundaryForCluster(chainID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// get the hash of the latest finalized block
		var lastFinalizedBlockID flow.Identifier
		err = operation.RetrieveNumberForCluster(chainID, boundary, &lastFinalizedBlockID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve latest finalized ID: %w", err)
		}

		// get the header of the parent of the new block
		var parent flow.Header
		err = operation.RetrieveHeader(block.ParentID, &parent)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve latest finalized header: %w", err)
		}

		// if the new block has a lower number than its parent, we can't add it
		if block.View <= parent.View {
			return fmt.Errorf("extending block height (%d) must be > parent height (%d)", block.View, parent.View)
		}

		// trace back from new block until we find a block that has the latest
		// finalized block as its parent
		for block.ParentID != lastFinalizedBlockID {

			// get the parent of current block
			err = operation.RetrieveHeader(block.ParentID, &block.Header)(tx)
			if err != nil {
				return fmt.Errorf("could not get parent (%x): %w", block.ParentID, err)
			}

			// if its number is below current boundary, the block does not connect
			// to the finalized protocol state and would break database consistency
			if block.View < boundary {
				return fmt.Errorf("block doesn't connect to finalized state")
			}
		}

		return nil
	})
}

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(genesis *cluster.Block) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// check chain ID
		if genesis.Header.ChainID != m.state.chainID {
			return fmt.Errorf("genesis chain ID (%s) does not match configured (%s)", genesis.Header.ChainID, m.state.chainID)
		}

		// check header number
		if genesis.Header.Height != 0 {
			return fmt.Errorf("genesis number should be 0 (got %d)", genesis.Header.Height)
		}

		// check header parent ID
		if genesis.Header.ParentID != flow.ZeroID {
			return fmt.Errorf("genesis parent ID must be zero hash (got %x)", genesis.Header.ParentID)
		}

		// check payload
		collSize := len(genesis.Payload.Collection.Transactions)
		if collSize != 0 {
			return fmt.Errorf("genesis collection should contain no transactions (got %d)", collSize)
		}

		// insert the block
		err := procedure.InsertClusterBlock(genesis)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block: %w", err)
		}

		// insert block number -> ID mapping
		err = operation.IndexClusterBlockHeight(genesis.Header.ChainID, genesis.Header.Height, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis number: %w", err)
		}

		// insert boundary
		err = operation.InsertClusterFinalizedHeight(genesis.Header.ChainID, genesis.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis boundary: %w", err)
		}

		return nil
	})
}

func (m *Mutator) Extend(block *cluster.Block) error {
	err := m.state.db.View(func(tx *badger.Txn) error {

		// check chain ID
		if block.Header.ChainID != m.state.chainID {
			return fmt.Errorf("new block chain ID (%s) does not match configured (%s)", block.Header.ChainID, m.state.chainID)
		}

		// get the chain ID, which determines which cluster state to query
		chainID := block.Header.ChainID

		// get the latest finalized block
		var final flow.Header
		err := procedure.RetrieveLatestFinalizedClusterHeader(chainID, &final)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized head: %w", err)
		}

		// get the header of the parent of the new block
		parent, err := m.state.headers.ByBlockID(block.Header.ParentID)
		if err != nil {
			return fmt.Errorf("could not retrieve latest finalized header: %w", err)
		}

		// the extending block must increase height by 1 from parent
		if block.Header.Height != parent.Height+1 {
			return fmt.Errorf("extending block height (%d) must be parent height + 1 (%d)", block.Header.Height, parent.Height)
		}

		// ensure that the extending block connects to the finalized state, we
		// do this by tracing back until we see a parent block that is the
		// latest finalized block, or reach height below the finalized boundary

		// start with the extending block's parent
		parentID := block.Header.ParentID
		for parentID != final.ID() {

			// get the parent of current block
			ancestor, err := m.state.headers.ByBlockID(parentID)
			if err != nil {
				return fmt.Errorf("could not get parent (%x): %w", block.Header.ParentID, err)
			}

			// if its number is below current boundary, the block does not connect
			// to the finalized protocol state and would break database consistency
			if ancestor.Height < final.Height {
				return fmt.Errorf("block doesn't connect to finalized state")
			}

			parentID = ancestor.ParentID
		}

		// we go back at most 1k blocks to check payload for now
		//TODO look back based on reference block ID and expiry
		// ref: https://github.com/dapperlabs/flow-go/issues/3556
		limit := block.Header.Height - 1000
		if limit > block.Header.Height { // overflow check
			limit = 0
		}

		// check for duplicate transactions in block's ancestry
		txLookup := make(map[flow.Identifier]struct{})
		for _, tx := range block.Payload.Collection.Transactions {
			txLookup[tx.ID()] = struct{}{}
		}

		var duplicateTxIDs flow.IdentifierList
		ancestorID := block.Header.ParentID
		for {
			ancestor, err := m.state.headers.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not retrieve ancestor header: %w", err)
			}

			if ancestor.Height <= limit {
				break
			}

			payload, err := m.state.payloads.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not retrieve ancestor payload: %w", err)
			}

			for _, tx := range payload.Collection.Transactions {
				txID := tx.ID()
				_, duplicated := txLookup[txID]
				if duplicated {
					duplicateTxIDs = append(duplicateTxIDs, txID)
				}
			}
			ancestorID = ancestor.ParentID
		}

		// if we have duplicate transactions, fail
		if len(duplicateTxIDs) > 0 {
			return fmt.Errorf("payload includes duplicate transactions (duplicates: %s)", duplicateTxIDs)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not validate extending block: %w", err)
	}

	// insert the block
	err = operation.RetryOnConflict(m.state.db.Update, procedure.InsertClusterBlock(block))
	if err != nil {
		return fmt.Errorf("could not insert extending block: %w", err)
	}

	return nil
}

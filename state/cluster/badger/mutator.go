package badger

import (
	"errors"
	"fmt"
	"math"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(genesis *cluster.Block) error {

	// check constraints
	err := m.state.db.View(func(tx *badger.Txn) error {

		// check chain ID
		if genesis.Header.ChainID != m.state.clusterID {
			return fmt.Errorf("genesis chain ID (%s) does not match configured (%s)", genesis.Header.ChainID, m.state.clusterID)
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

		return nil
	})
	if err != nil {
		return err
	}

	// bootstrap cluster state
	err = operation.RetryOnConflict(m.state.db.Update, func(tx *badger.Txn) error {

		chainID := genesis.Header.ChainID
		// insert the block
		err := procedure.InsertClusterBlock(genesis)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block: %w", err)
		}
		// insert block number -> ID mapping
		err = operation.IndexClusterBlockHeight(chainID, genesis.Header.Height, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis number: %w", err)
		}
		// insert boundary
		err = operation.InsertClusterFinalizedHeight(chainID, genesis.Header.Height)(tx)
		// insert started view for hotstuff
		if err != nil {
			return fmt.Errorf("could not insert genesis boundary: %w", err)
		}
		err = operation.InsertStartedView(chainID, genesis.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		// insert voted view for hotstuff
		err = operation.InsertVotedView(chainID, genesis.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}

		return nil
	})
	return err
}

func (m *Mutator) Extend(block *cluster.Block) error {
	err := m.state.db.View(func(tx *badger.Txn) error {

		header := block.Header
		payload := block.Payload

		// check chain ID
		if header.ChainID != m.state.clusterID {
			return state.NewInvalidExtensionErrorf("new block chain ID (%s) does not match configured (%s)", block.Header.ChainID, m.state.clusterID)
		}

		// get the chain ID, which determines which cluster state to query
		chainID := header.ChainID

		// get the latest finalized block
		var final flow.Header
		err := procedure.RetrieveLatestFinalizedClusterHeader(chainID, &final)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized head: %w", err)
		}

		// get the header of the parent of the new block
		parent, err := m.state.headers.ByBlockID(header.ParentID)
		if err != nil {
			return fmt.Errorf("could not retrieve latest finalized header: %w", err)
		}

		// the extending block must increase height by 1 from parent
		if header.Height != parent.Height+1 {
			return state.NewInvalidExtensionErrorf("extending block height (%d) must be parent height + 1 (%d)",
				block.Header.Height, parent.Height)
		}

		// ensure that the extending block connects to the finalized state, we
		// do this by tracing back until we see a parent block that is the
		// latest finalized block, or reach height below the finalized boundary

		// start with the extending block's parent
		parentID := header.ParentID
		for parentID != final.ID() {

			// get the parent of current block
			ancestor, err := m.state.headers.ByBlockID(parentID)
			if err != nil {
				return fmt.Errorf("could not get parent (%x): %w", block.Header.ParentID, err)
			}

			// if its number is below current boundary, the block does not connect
			// to the finalized protocol state and would break database consistency
			if ancestor.Height < final.Height {
				return state.NewOutdatedExtensionErrorf("block doesn't connect to finalized state. ancestor.Height (%v), final.Height (%v)",
					ancestor.Height, final.Height)
			}

			parentID = ancestor.ParentID
		}

		// check that all transactions within the collection are valid
		minRefID := flow.ZeroID
		minRefHeight := uint64(math.MaxUint64)
		for _, flowTx := range payload.Collection.Transactions {
			refBlock, err := m.state.headers.ByBlockID(flowTx.ReferenceBlockID)
			if errors.Is(err, storage.ErrNotFound) {
				// unknown reference blocks are invalid
				return state.NewInvalidExtensionErrorf("unknown reference block (id=%x): %v", flowTx.ReferenceBlockID, err)
			}
			if err != nil {
				return fmt.Errorf("could not check reference block (id=%x): %w", flowTx.ReferenceBlockID, err)
			}

			if refBlock.Height < minRefHeight {
				minRefHeight = refBlock.Height
				minRefID = flowTx.ReferenceBlockID
			}
		}

		// a valid collection must reference the oldest reference block among
		// its constituent transactions
		if payload.Collection.Len() > 0 && minRefID != payload.ReferenceBlockID {
			return state.NewInvalidExtensionErrorf(
				"reference block (id=%x) must match oldest transaction's reference block (id=%x)",
				payload.ReferenceBlockID, minRefID,
			)
		}

		// a valid collection must reference a valid reference block
		// NOTE: it is valid for a collection to be expired at this point,
		// otherwise we would compromise liveness of the cluster.
		refBlock, err := m.state.headers.ByBlockID(payload.ReferenceBlockID)
		if errors.Is(err, storage.ErrNotFound) {
			return state.NewInvalidExtensionErrorf("unknown reference block (id=%x)", payload.ReferenceBlockID)
		}
		if err != nil {
			return fmt.Errorf("could not check reference block: %w", err)
		}

		// TODO ensure the reference block is part of the main chain
		_ = refBlock

		// we go back a fixed number of  blocks to check payload for now
		// TODO look back based on reference block ID and expiry https://github.com/dapperlabs/flow-go/issues/3556
		limit := block.Header.Height - flow.DefaultTransactionExpiry
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
			return state.NewInvalidExtensionErrorf("payload includes duplicate transactions (duplicates: %s)",
				duplicateTxIDs)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not validate extending block: %w", err)
	}

	// insert the new block
	err = m.state.db.Update(procedure.InsertClusterBlock(block))
	if err != nil {
		return fmt.Errorf("could not insert cluster block: %w", err)
	}
	return nil
}

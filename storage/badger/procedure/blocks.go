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

		// if we have a height, this is not genesis and we can return
		if header.Height > 0 {
			return nil
		}

		// add identities for genesis block
		var identities flow.IdentityList
		err = operation.RetrieveIdentities(&identities)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve identities: %w", err)
		}

		block.Payload.Identities = identities

		return nil
	}
}

// Bootstrap inserts the genesis block to the storage
func Bootstrap(commit flow.StateCommitment, genesis *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// 1) insert the block, the genesis identities and index it by beight
		genesisID := genesis.ID()
		err := InsertBlock(genesisID, genesis)(tx)
		if err != nil {
			return fmt.Errorf("could not insert header: %w", err)
		}
		err = operation.InsertIdentities(genesis.Payload.Identities)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis identities: %w", err)
		}
		err = operation.IndexBlockHeight(0, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not initialize boundary: %w", err)
		}

		// TODO: put seal into payload to have it signed

		// 3) generate genesis execution result, insert and index by block
		result := flow.ExecutionResult{ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: flow.ZeroID,
			BlockID:          genesis.ID(),
			FinalStateCommit: commit,
		}}
		err = operation.InsertExecutionResult(&result)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis result: %w", err)
		}
		err = operation.IndexExecutionResult(genesis.ID(), result.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index genesis result: %w", err)
		}

		// 4) generate genesis block seal, insert and index by block
		seal := flow.Seal{
			BlockID:      genesis.ID(),
			ResultID:     result.ID(),
			InitialState: flow.GenesisStateCommitment,
			FinalState:   result.FinalStateCommit,
		}
		err = operation.InsertSeal(seal.ID(), &seal)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis seal: %w", err)
		}
		err = operation.IndexBlockSeal(genesis.ID(), seal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index genesis block seal: %w", err)
		}

		// 5) initialize all of the special views and heights
		err = operation.InsertStartedView(genesis.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertVotedView(genesis.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertFinalizedHeight(genesis.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert finalized height: %w", err)
		}
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

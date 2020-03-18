package observation

import (
	"strings"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// BlockChainState represents the entire block chain including blocks, collections and transactions
type BlockchainState struct {
	headers     storage.Headers
	collections storage.Collections
}

func NewBlockchainState(h storage.Headers, c storage.Collections) *BlockchainState {
	return &BlockchainState{
		headers:     h,
		collections: c,
	}
}

// StoreCollection adds a collection to the block chain state if not already present
func (b *BlockchainState) StoreCollection(collection *flow.Collection) error {

	err := b.collections.Store(collection)

	// since the transactions in a collection remain same collection, it just needs to be added once
	if err != nil && strings.Contains(err.Error(), storage.ErrAlreadyExists.Error()) {
		return nil
	}
	return err
}

// Block retrieves a Block by ID
func (b *BlockchainState) Block(id flow.Identifier) (*flow.Header, error) {
	return b.headers.ByBlockID(id)
}

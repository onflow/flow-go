package observation

import (
	"strings"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// BlockChainState represents the entire block chain including blocks, collections and transactions
type BlockchainState struct {
	headers      storage.Headers
	collections  storage.Collections
	transactions storage.Transactions
}

func NewBlockchainState(h storage.Headers, c storage.Collections, t storage.Transactions) *BlockchainState {
	return &BlockchainState{
		headers:      h,
		collections:  c,
		transactions: t,
	}
}

// StoreCollection adds a collection to the block chain state if not already present
func (b *BlockchainState) StoreCollection(collection *flow.Collection) error {

	// store the collection minus the transaction details (transactions are stored separately)
	light := collection.Light()
	err := b.collections.StoreLight(&light)

	// since the transactions in a collection remain same, it just needs to be added once
	if err != nil && strings.Contains(err.Error(), storage.ErrAlreadyExists.Error()) {
		return nil
	}

	return err
}

// AddTransaction adds the transaction body to the state if not present
func (b *BlockchainState) AddTransaction(transaction *flow.TransactionBody) error {

	err := b.transactions.Store(transaction)

	// the transaction may have been already added if this obs node earlier received a SendTransaction
	if err != nil {
		if !strings.Contains(err.Error(), storage.ErrAlreadyExists.Error()) {
			return err
		}
	}
	return nil
}

// AddTransactionCollectionMapping adds the transaction id to collection id record
func (b *BlockchainState) AddTransactionCollectionMapping(transaction *flow.TransactionBody, collID flow.Identifier) error {
	return b.transactions.StoreCollectionID(transaction.ID(), collID)
}

// Block retrieves a Block by ID
func (b *BlockchainState) Block(id flow.Identifier) (*flow.Header, error) {
	return b.headers.ByBlockID(id)
}

// GetTransaction retrieves a transaction
func (b *BlockchainState) GetTransaction(id flow.Identifier) (*flow.TransactionBody, error) {
	return b.transactions.ByID(id)
	// TODO: send a transaction request to the collection node and wait for a response
}
